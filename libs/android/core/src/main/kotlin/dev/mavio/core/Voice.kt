package dev.mavio.core

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageManager
import android.media.AudioAttributes
import android.media.AudioDeviceInfo
import android.media.AudioFormat
import android.media.AudioManager
import android.media.AudioRecord
import android.media.AudioTrack
import android.media.MediaRecorder
import android.media.audiofx.AcousticEchoCanceler
import android.media.audiofx.NoiseSuppressor
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.atomic.AtomicInteger
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sqrt
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import okio.ByteString.Companion.toByteString
import org.json.JSONObject

/**
 * A voice session from the client's side (docs/assistant.md §8, §9.4): the
 * microphone captured by `AudioRecord` from `VOICE_COMMUNICATION`, with the
 * echo canceler and noise suppressor, as 16 kHz mono 16-bit PCM in 20 ms
 * frames, sent over the session's WebSocket; the answers' speech played by
 * an `AudioTrack` of `USAGE_VOICE_COMMUNICATION`, whose echo is canceled.
 * The microphone opens at once and what it hears before the WebSocket
 * opens is kept and sent in order. Its handlers run on the main thread,
 * where it is used.
 */
class VoiceSession private constructor(
    private val context: Context,
    private val handlers: Handlers,
    first: AudioRecord,
) {
    /** What a session tells as it goes. */
    class Handlers(
        /** A JSON event of the session (§8.6). */
        val event: (JSONObject) -> Unit,
        /** The session ended: unreachable when it closed before it was ready, error when it failed. */
        val closed: (unreachable: Boolean, error: String?) -> Unit,
        /** The microphone's audio has started coming. */
        val capturing: () -> Unit,
        /** The microphone gives nothing but digital silence; its name, if known. */
        val silent: (device: String) -> Unit,
    )

    /** Why the microphone could not be opened. */
    sealed class Failure(message: String) : Exception(message) {
        /** The user refused the microphone. */
        class Denied : Failure("denied")

        /** There is none, or it failed to open. */
        class Unavailable(reason: String) : Failure(reason)
    }

    private val main = Handler(Looper.getMainLooper())
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val audio = context.getSystemService(AudioManager::class.java)
    // A phone plays the answers as a call, at the call's volume its keys set
    // during the session; a device without an earpiece, as a TV, as its
    // assistant, at the media's volume its remote sets.
    private val usage = if (audio.getDevices(AudioManager.GET_DEVICES_OUTPUTS).any { it.type == AudioDeviceInfo.TYPE_BUILTIN_EARPIECE }) {
        AudioAttributes.USAGE_VOICE_COMMUNICATION
    } else {
        AudioAttributes.USAGE_ASSISTANT
    }
    private var record = first
    private var effects = effects(first)
    // The times the microphone was opened again since it last gave audio.
    private var reopens = 0
    private var socket: WebSocket? = null
    private var opened = false
    private val pending = ArrayList<ByteArray>()
    private var ended = false
    private var captured = false
    // 20 ms frames of digital silence, until one was not.
    private var zeros = 0
    // The frames the microphone gave, and when it gave the last, in uptime
    // milliseconds: told by the capture thread, so that a busy main thread
    // with frames waiting for it does not take them for a gap.
    private var frames = 0
    @Volatile private var lastFrameAt = 0L
    // Whether the session said it is ready: one closed before failed.
    private var ready = false
    // Push to talk: whether the user talks, and what the socket was told.
    private var talking = false
    private var told = false

    /** Whether the session listens only while the user holds to talk. */
    var holdsToTalk = false
        private set

    @Volatile private var inLevel = 0.0
    private val speaker = Speaker()
    private val previousMode = audio.mode
    private var routed = false

    /** How loud the microphone is now, from 0 to about 1. */
    fun inputLevel(): Double = if (ended) 0.0 else inLevel

    /** How loud the answer's speech is now, from 0 to about 1. */
    fun outputLevel(): Double = if (ended) 0.0 else speaker.level()

    /** How much of the answer's speech has played and has come, in milliseconds. */
    fun speech(): Pair<Double, Double> = speaker.speech()

    private fun start() {
        // A phone plays a call's audio from its earpiece: the answers come
        // from its speaker, unless a headset is on.
        if (Build.VERSION.SDK_INT >= 31) {
            val devices = audio.availableCommunicationDevices
            val headset = devices.any { it.type in HEADSETS }
            val speakerDevice = devices.firstOrNull { it.type == AudioDeviceInfo.TYPE_BUILTIN_SPEAKER }
            if (devices.any { it.type == AudioDeviceInfo.TYPE_BUILTIN_EARPIECE } && !headset && speakerDevice != null) {
                audio.mode = AudioManager.MODE_IN_COMMUNICATION
                routed = audio.setCommunicationDevice(speakerDevice)
            }
        }
        begin()
        speaker.start()
        // A microphone that gives nothing at all, as a remote's does until
        // its own key turns it on, is told as one that gives silence.
        main.postDelayed({
            if (!ended && frames == 0) {
                zeros = -1
                handlers.silent(record.routedDevice?.productName?.toString().orEmpty())
            }
        }, SILENT_FRAMES * 20L)
    }

    // Starts the microphone and reads it on a thread of its own, or opens it again.
    private fun begin() {
        val r = record
        runCatching { r.startRecording() }
        if (r.recordingState != AudioRecord.RECORDSTATE_RECORDING) {
            reopen(r)
            return
        }
        Thread({ capture(r) }, "mavio-voice-capture").start()
    }

    // The microphone failed to start or stopped giving audio: as a TV's
    // remote does when the TV's own assistant takes it, or the TV's audio
    // service restarts. It is opened again a few times before the session
    // ends.
    private fun reopen(failed: AudioRecord) {
        if (ended || record !== failed) return
        if (++reopens > REOPENS) {
            end(error = "the microphone stopped")
            return
        }
        effects.forEach { it.release() }
        effects = emptyList()
        runCatching { failed.stop() }
        failed.release()
        main.postDelayed({
            if (ended || record !== failed) return@postDelayed
            val r = runCatching { open() }.getOrNull()
            if (r == null) {
                reopen(failed)
                return@postDelayed
            }
            record = r
            effects = effects(r)
            begin()
        }, REOPEN_MS)
    }

    // Reads the microphone's 20 ms frames until the session ends or the microphone is replaced.
    private fun capture(r: AudioRecord) {
        val frame = ShortArray(FRAME)
        while (!ended && record === r) {
            var read = 0
            while (read < FRAME && !ended) {
                val n = r.read(frame, read, FRAME - read)
                if (n <= 0) break
                read += n
            }
            if (read < FRAME) {
                // The microphone stopped giving audio.
                main.post { reopen(r) }
                break
            }
            var sum = 0.0
            var heard = false
            val bytes = ByteArray(FRAME * 2)
            for (i in 0 until FRAME) {
                val s = frame[i].toInt()
                if (s != 0) heard = true
                sum += (s / 32768.0) * (s / 32768.0)
                bytes[i * 2] = s.toByte()
                bytes[i * 2 + 1] = (s shr 8).toByte()
            }
            inLevel = loudness(sum / FRAME)
            lastFrameAt = SystemClock.uptimeMillis()
            main.post { audio(bytes, heard) }
        }
    }

    private fun audio(pcm: ByteArray, heard: Boolean) {
        if (ended) return
        if (frames == 0) main.postDelayed(::fill, FRAME_MS)
        frames++
        reopens = 0
        // Listening is told once the microphone gives more than digital silence.
        if (!captured && heard) {
            captured = true
            handlers.capturing()
        }
        if (zeros >= 0) {
            zeros = if (heard) -1 else zeros + 1
            // A real microphone's noise is never all zeros for 2 s.
            if (zeros == SILENT_FRAMES) {
                zeros = -1
                handlers.silent(record.routedDevice?.productName?.toString().orEmpty())
            }
        }
        // The socket opens once the plugin has made the session's services,
        // which can take seconds: what the user says meanwhile is kept.
        if (opened) socket?.send(pcm.toByteString()) else pending += pcm
    }

    // A remote's microphone gives audio only while its key is held: the
    // gaps between are sent as silence, so that the session hears the user
    // fall silent and ends the turn.
    private fun fill() {
        if (ended) return
        if (SystemClock.uptimeMillis() - lastFrameAt > GAP_MS) {
            val pcm = ByteArray(FRAME * 2)
            inLevel = 0.0
            if (opened) socket?.send(pcm.toByteString()) else pending += pcm
        }
        main.postDelayed(::fill, FRAME_MS)
    }

    /** Connects to the session's WebSocket, once [url] tells it. */
    fun connect(pushToTalk: Boolean, url: suspend () -> String) {
        holdsToTalk = pushToTalk
        scope.launch {
            val u = try {
                url()
            } catch (e: Exception) {
                end(error = e.message ?: e.toString())
                return@launch
            }
            open(u)
        }
    }

    private fun open(url: String) {
        if (ended) return
        socket = CLIENT.newWebSocket(
            Request.Builder().url(url).build(),
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) = post(webSocket) { socketOpened() }

                override fun onMessage(webSocket: WebSocket, text: String) = post(webSocket) { message(text) }

                override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                    val pcm = bytes.toByteArray()
                    post(webSocket) { if (!ended) speaker.push(pcm) }
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = post(webSocket) { end(unreachable = !ready) }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) = post(webSocket) { end(unreachable = !ready) }
            },
        )
    }

    // Runs on the main thread what the socket of the session tells.
    private fun post(webSocket: WebSocket, block: () -> Unit) {
        main.post { if (socket === webSocket) block() }
    }

    private fun socketOpened() {
        if (ended) return
        opened = true
        tell()
        pending.forEach { socket?.send(it.toByteString()) }
        pending.clear()
    }

    /** Push to talk: the user holds to talk, or lets go. */
    fun talk(on: Boolean) {
        talking = on
        if (on) speaker.drop()
        if (opened) tell()
    }

    /** Sends an event, once the socket is open. */
    fun send(event: JSONObject) {
        if (opened) socket?.send(event.toString())
    }

    /** Ends the session. */
    fun close() {
        send(JSONObject().put("type", "close"))
        ready = true
        end()
    }

    private fun tell() {
        if (!holdsToTalk || talking == told) return
        told = talking
        send(JSONObject().put("type", if (talking) "ptt_start" else "ptt_end"))
    }

    private fun message(text: String) {
        val event = runCatching { JSONObject(text) }.getOrNull() ?: return
        when (event.optString("type")) {
            "ready" -> {
                ready = true
                val r = event.optDouble("output_sample_rate", 0.0)
                if (r > 0) speaker.rate(r.toInt())
            }
            // What the run plays is counted from here.
            "run" -> speaker.run()
            // The relay keeps the answer's speech as far as it was played (§8.8).
            "interrupt" -> send(JSONObject().put("type", "played").put("ms", speaker.drop()))
        }
        handlers.event(event)
    }

    private fun end(unreachable: Boolean = false, error: String? = null) {
        if (ended) return
        ended = true
        release()
        socket?.close(1000, null)
        socket = null
        scope.cancel()
        handlers.closed(unreachable, error)
    }

    private fun release() {
        runCatching { record.stop() }
        runCatching { record.release() }
        effects.forEach { it.release() }
        speaker.release()
        if (routed && Build.VERSION.SDK_INT >= 31) audio.clearCommunicationDevice()
        if (routed) audio.mode = previousMode
    }

    /**
     * Plays the answers' speech, 16-bit PCM at the session's rate, from a
     * thread of its own, counting the samples of the run queued and played.
     */
    private inner class Speaker {
        private var rate = 24000
        @Volatile private var track = track(rate)
        private val queue = LinkedBlockingQueue<Pair<Int, ShortArray>>()
        // Bumped by a drop, so that the writer skips what it stopped.
        private val generation = AtomicInteger()
        // Samples written since the last drop, the head's position then, and
        // where the run started; each chunk's end and loudness.
        private var written = 0L
        private var base = 0L
        private var runStart = 0L
        private val levels = ArrayDeque<Pair<Long, Double>>()
        @Volatile private var stopped = false

        fun start() {
            track.play()
            Thread({ write() }, "mavio-voice-playback").start()
        }

        private fun write() {
            while (!stopped) {
                val (gen, samples) = runCatching { queue.take() }.getOrNull() ?: return
                if (stopped) return
                var at = 0
                while (at < samples.size && gen == generation.get() && !stopped) {
                    // A track replaced or released meanwhile fails the write.
                    val n = runCatching { track.write(samples, at, min(samples.size - at, FRAME * 2)) }.getOrDefault(0)
                    if (n <= 0) break
                    at += n
                }
            }
        }

        // Samples played since the last drop.
        private fun played(): Long = (track.playbackHeadPosition.toLong() and 0xFFFFFFFFL) - base

        fun push(pcm: ByteArray) {
            val count = pcm.size / 2
            if (count == 0) return
            val samples = ShortArray(count)
            var sum = 0.0
            for (i in 0 until count) {
                val s = ((pcm[i * 2 + 1].toInt() shl 8) or (pcm[i * 2].toInt() and 0xFF)).toShort()
                samples[i] = s
                sum += (s / 32768.0) * (s / 32768.0)
            }
            written += count
            levels.addLast(written to loudness(sum / count))
            queue.put(generation.get() to samples)
        }

        fun level(): Double {
            val p = played()
            while (levels.isNotEmpty() && levels.first().first <= p) levels.removeFirst()
            return levels.firstOrNull()?.second ?: 0.0
        }

        fun speech(): Pair<Double, Double> {
            val p = played().coerceIn(0, written)
            return (p - runStart) * 1000.0 / rate to (written - runStart) * 1000.0 / rate
        }

        fun run() {
            runStart = played().coerceIn(0, written)
        }

        /** Drops what was not played, returning how much of the run's speech was, in milliseconds. */
        fun drop(): Int {
            val p = played().coerceIn(0, written)
            val ms = ((p - runStart) * 1000.0 / rate).roundToInt()
            generation.incrementAndGet()
            queue.clear()
            levels.clear()
            runCatching {
                track.pause()
                track.flush()
                track.play()
            }
            base = track.playbackHeadPosition.toLong() and 0xFFFFFFFFL
            written = 0
            runStart = 0
            return ms
        }

        /** Plays at the session's rate, from its ready event, while nothing is queued. */
        fun rate(r: Int) {
            if (r == rate || played() < written) return
            rate = r
            generation.incrementAndGet()
            queue.clear()
            val old = track
            track = track(r)
            track.play()
            old.release()
            base = 0
            written = 0
            runStart = 0
        }

        fun release() {
            stopped = true
            generation.incrementAndGet()
            queue.clear()
            // Wakes the writer.
            queue.put(0 to ShortArray(0))
            runCatching { track.stop() }
            track.release()
        }

        private fun track(rate: Int): AudioTrack = AudioTrack.Builder()
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(usage)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                    .build(),
            )
            .setAudioFormat(
                AudioFormat.Builder()
                    .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                    .setSampleRate(rate)
                    .setChannelMask(AudioFormat.CHANNEL_OUT_MONO)
                    .build(),
            )
            .setTransferMode(AudioTrack.MODE_STREAM)
            .setBufferSizeInBytes(maxOf(AudioTrack.getMinBufferSize(rate, AudioFormat.CHANNEL_OUT_MONO, AudioFormat.ENCODING_PCM_16BIT), rate / 5 * 2))
            .setSessionId(record.audioSessionId)
            .build()
    }

    companion object {
        // 20 ms at 16 kHz.
        private const val FRAME = 320

        // How many 20 ms frames of digital silence tell the microphone is silent.
        private const val SILENT_FRAMES = 100

        // A frame's length, and how long without one is a gap filled with silence, in milliseconds.
        private const val FRAME_MS = 20L
        private const val GAP_MS = 100L

        // How many times the microphone is opened again, and how long after it failed.
        private const val REOPENS = 5
        private const val REOPEN_MS = 600L

        private val HEADSETS = setOf(
            AudioDeviceInfo.TYPE_WIRED_HEADSET,
            AudioDeviceInfo.TYPE_WIRED_HEADPHONES,
            AudioDeviceInfo.TYPE_USB_HEADSET,
            AudioDeviceInfo.TYPE_BLUETOOTH_SCO,
            AudioDeviceInfo.TYPE_BLE_HEADSET,
        )

        private val CLIENT = OkHttpClient()

        /** Whether the app may use the microphone. */
        fun permitted(context: Context): Boolean =
            context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED

        /** Opens the microphone; fails when it is refused or missing. */
        fun capture(context: Context, handlers: Handlers): VoiceSession {
            if (!permitted(context)) throw Failure.Denied()
            return VoiceSession(context.applicationContext, handlers, open()).also {
                try {
                    it.start()
                } catch (e: Exception) {
                    it.release()
                    throw Failure.Unavailable(e.message ?: e.toString())
                }
            }
        }

        // Opens the first source that has a microphone.
        @SuppressLint("MissingPermission")
        private fun open(): AudioRecord {
            val size = maxOf(
                AudioRecord.getMinBufferSize(16000, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT),
                FRAME * 2 * 10,
            )
            // Voice communication's echo canceler first; a device that
            // routes it to no microphone may still route recognition's or
            // the plain microphone's, its own echo canceler attached after.
            var failure: String? = null
            val record = SOURCES.firstNotNullOfOrNull { source ->
                try {
                    AudioRecord.Builder()
                        .setAudioSource(source)
                        .setAudioFormat(
                            AudioFormat.Builder()
                                .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                                .setSampleRate(16000)
                                .setChannelMask(AudioFormat.CHANNEL_IN_MONO)
                                .build(),
                        )
                        .setBufferSizeInBytes(size)
                        .build()
                        .let { r ->
                            if (r.state == AudioRecord.STATE_INITIALIZED) {
                                r
                            } else {
                                r.release()
                                null
                            }
                        }
                } catch (e: Exception) {
                    failure = e.message ?: e.toString()
                    null
                }
            } ?: throw Failure.Unavailable(failure ?: "no microphone")
            return record
        }

        // The echo canceler and noise suppressor of a microphone, where the device has them.
        private fun effects(record: AudioRecord) = listOfNotNull(
            if (AcousticEchoCanceler.isAvailable()) AcousticEchoCanceler.create(record.audioSessionId)?.apply { enabled = true } else null,
            if (NoiseSuppressor.isAvailable()) NoiseSuppressor.create(record.audioSessionId)?.apply { enabled = true } else null,
        )

        // The sources tried in turn.
        private val SOURCES = listOf(
            MediaRecorder.AudioSource.VOICE_COMMUNICATION,
            MediaRecorder.AudioSource.VOICE_RECOGNITION,
            MediaRecorder.AudioSource.MIC,
        )

        /** How loud speech is, from 0 to about 1: its RMS of roughly 0.01 to 0.3 spread out by its square root. */
        private fun loudness(meanSquare: Double): Double = min(1.0, sqrt(sqrt(meanSquare)) * 1.5)
    }
}

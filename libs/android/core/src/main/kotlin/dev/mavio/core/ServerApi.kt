package dev.mavio.core

import com.google.protobuf.MessageLite
import com.google.protobuf.Timestamp
import java.time.Instant

/**
 * Calls a unary procedure of a server signed in to as its session, for the
 * screens the core does not model, such as the settings, as the web UI calls
 * the server's API: procedure is its path, such as
 * "/mavio.system.v1.SystemService/GetSystemInfo", and parse reads the
 * response, such as GetSystemInfoResponse::parseFrom.
 */
suspend fun <T : MessageLite> Client.call(serverId: String, procedure: String, request: MessageLite, parse: (ByteArray) -> T): T =
    parse(callServer(serverId, procedure, request.toByteArray()).body)

/** The time, null when it is the zero timestamp a message leaves out. */
val Timestamp.instant: Instant? get() = if (seconds == 0L && nanos == 0) null else Instant.ofEpochSecond(seconds, nanos.toLong())

package dev.mavio.core

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Keeps the core's secrets — each server's session and the device key —
 * encrypted with an AES key of the Android Keystore, which never leaves it,
 * in the app's private preferences, one entry per ID.
 */
class KeystoreTokens(context: Context, private val alias: String = "dev.mavio.tokens") : SecureTokens {
    private val prefs = context.getSharedPreferences("tokens", Context.MODE_PRIVATE)

    private fun key(): SecretKey {
        val store = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (store.getKey(alias, null) as SecretKey?)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build(),
        )
        return generator.generateKey()
    }

    @Synchronized
    override fun token(serverId: String): String? {
        val saved = prefs.getString(serverId, null) ?: return null
        return try {
            val bytes = Base64.decode(saved, Base64.NO_WRAP)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, IV))
            String(cipher.doFinal(bytes, IV, bytes.size - IV), Charsets.UTF_8)
        } catch (_: java.security.GeneralSecurityException) {
            // A key lost with the app's data restored elsewhere: the
            // secret is gone, and the server signs in again.
            null
        }
    }

    @Synchronized
    override fun setToken(serverId: String, token: String) {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val sealed = cipher.iv + cipher.doFinal(token.toByteArray(Charsets.UTF_8))
        prefs.edit().putString(serverId, Base64.encodeToString(sealed, Base64.NO_WRAP)).commit()
    }

    @Synchronized
    override fun deleteToken(serverId: String) {
        prefs.edit().remove(serverId).commit()
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV = 12
    }
}

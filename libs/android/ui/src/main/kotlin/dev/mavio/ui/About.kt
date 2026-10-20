package dev.mavio.ui

import android.os.Build
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.content.pm.PackageInfoCompat
import dev.mavio.core.AppModel
import dev.mavio.design.MavioSpacing

/**
 * About: the app, its name as the servers know it, its version and code,
 * the core's version and the license; and the device, its name and ID as
 * the servers know them, its maker, model, system and architecture.
 */
@Composable
fun AboutSettings(model: AppModel) {
    val context = LocalContext.current
    val app = remember {
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        "${info.versionName.orEmpty()} (${PackageInfoCompat.getLongVersionCode(info)})"
    }
    val device = model.device
    SettingsList(stringResource(R.string.settings_about)) {
        item {
            Group(stringResource(R.string.about_app)) {
                AboutRow(stringResource(R.string.about_app_name), device.client)
                RowDivider()
                AboutRow(stringResource(R.string.about_client_version), app)
                RowDivider()
                AboutRow(stringResource(R.string.about_core_version), dev.mavio.core.version)
                RowDivider()
                AboutRow(stringResource(R.string.about_license), "GPL-3.0")
            }
        }
        item {
            Group(stringResource(R.string.about_device), stringResource(R.string.about_device_footer)) {
                AboutRow(stringResource(R.string.about_device_name), device.name)
                RowDivider()
                AboutRow(stringResource(R.string.about_device_id), device.id)
                RowDivider()
                AboutRow(stringResource(R.string.about_manufacturer), Build.MANUFACTURER)
                RowDivider()
                AboutRow(stringResource(R.string.about_model), "${Build.MODEL} (${Build.DEVICE})")
                RowDivider()
                AboutRow(stringResource(R.string.about_system), "Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
                RowDivider()
                AboutRow(stringResource(R.string.about_architecture), Build.SUPPORTED_ABIS.firstOrNull().orEmpty())
            }
        }
    }
}

/** A label and its value, which wraps when it is long, as a device's ID. */
@Composable
private fun AboutRow(label: String, value: String) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = if (LocalTelevision.current) 68.dp else 48.dp).padding(horizontal = rowPadding, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = theme.type.body.text)
        Text(
            value,
            Modifier.weight(1f).padding(start = MavioSpacing.s16),
            style = theme.type.body.text,
            color = theme.colors.textSecondary,
            textAlign = TextAlign.End,
        )
    }
}

package dev.mavio.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import dev.mavio.core.AppModel
import dev.mavio.core.localization
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.MavioButton
import dev.mavio.ui.R
import dev.mavio.ui.filterSections
import dev.mavio.ui.text
import dev.mavio.ui.theme
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Profile.MediaKind

/**
 * The sheet of a catalog's filters and sorts (docs/clients.md §6.3.28),
 * as the web UI's dialog, offering those the catalog takes.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun FiltersSheet(
    model: AppModel,
    serverId: String,
    catalog: Catalog,
    kind: MediaKind,
    initial: CatalogFilters,
    onDismiss: () -> Unit,
    onApply: (CatalogFilters) -> Unit,
) {
    var f by remember { mutableStateOf(initial) }
    val localization by produceState<Pair<List<Pair<String, String>>, List<Pair<String, String>>>?>(null, serverId) {
        value = model.localization(serverId)
    }
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismiss, sheetState = sheet, containerColor = theme.colors.surfaceRaised) {
        LazyColumn(
            Modifier.fillMaxWidth(),
            contentPadding = PaddingValues(horizontal = theme.layout.margin),
        ) {
            item {
                Text(stringResource(R.string.discover_filters), Modifier.padding(bottom = MavioSpacing.s8), style = theme.type.title2.text)
            }
            filterSections(serverId, catalog, kind, f, localization) { f = it }
            item {
                Row(
                    Modifier.fillMaxWidth().padding(vertical = MavioSpacing.s16).navigationBarsPadding(),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                ) {
                    MavioButton(stringResource(R.string.discover_reset), { f = CatalogFilters() }, Modifier.weight(1f), kind = ButtonKind.Secondary, large = true)
                    MavioButton(stringResource(R.string.discover_apply), { onApply(f) }, Modifier.weight(1f), large = true)
                }
            }
            item { Spacer(Modifier.height(MavioSpacing.s8)) }
        }
    }
}

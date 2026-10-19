package dev.mavio.core

import java.text.Collator
import java.util.Locale
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import mavio.system.v1.Localization.ListCountriesResponse
import mavio.system.v1.Localization.ListLanguagesResponse
import mavio.system.v1.listCountriesRequest
import mavio.system.v1.listLanguagesRequest

private fun localizationProcedure(method: String) = "/mavio.system.v1.LocalizationService/$method"

/**
 * The languages and countries of a server's metadata, as code and name,
 * named in the user's language where it can, by name, without repeats.
 */
suspend fun AppModel.localization(serverId: String): Pair<List<Pair<String, String>>, List<Pair<String, String>>> = coroutineScope {
    val langs = async {
        runCatching {
            client.call(serverId, localizationProcedure("ListLanguages"), listLanguagesRequest {}, ListLanguagesResponse::parseFrom)
        }.getOrNull()
    }
    val ctrs = async {
        runCatching {
            client.call(serverId, localizationProcedure("ListCountries"), listCountriesRequest {}, ListCountriesResponse::parseFrom)
        }.getOrNull()
    }
    val locale = Locale.getDefault()
    val collator = Collator.getInstance(locale)
    fun named(list: List<Pair<String, String>>, of: (String) -> String) = list
        .filter { it.first.isNotEmpty() }
        .distinctBy { it.first }
        .map { (code, name) -> code to (of(code).takeIf { it.isNotEmpty() && it != code } ?: name) }
        .sortedWith { a, b -> collator.compare(a.second, b.second) }
    val languages = named(langs.await()?.languagesList?.map { it.code to it.name }.orEmpty()) { Locale.forLanguageTag(it).getDisplayLanguage(locale) }
    val countries = named(ctrs.await()?.countriesList?.map { it.code to it.name }.orEmpty()) { Locale("", it).getDisplayCountry(locale) }
    languages to countries
}

/** The choices of a list, with the value chosen though the list lacks it. */
fun choices(list: List<Pair<String, String>>, chosen: String): List<Pair<String, String>> =
    if (chosen.isEmpty() || list.any { it.first == chosen }) list else list + (chosen to chosen)

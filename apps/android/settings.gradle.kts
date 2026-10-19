pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode = RepositoriesMode.FAIL_ON_PROJECT_REPOS
    repositories {
        google()
        mavenCentral()
    }
    versionCatalogs {
        create("libs") { from(files("../../libs/android/gradle/libs.versions.toml")) }
    }
}

rootProject.name = "mavio-android"
includeBuild("../../libs/android")
// Generates the app's baseline profile and measures its start.
include(":baselineprofile")

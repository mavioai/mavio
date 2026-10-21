import org.gradle.api.file.ArchiveOperations
import org.gradle.process.ExecOperations
import javax.inject.Inject

// The shared core's Kotlin API with the app services above it. The API and
// the core's libraries come from scripts/build-core.sh, in build/rust.
plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "dev.mavio.core"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
        // The rules an app's R8 keeps this library's code by.
        consumerProguardFiles("consumer-rules.pro")
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    testOptions.unitTests.all {
        // The core built for the machine running the tests.
        it.systemProperty("jna.library.path", file("build/rust/host").absolutePath)
    }
}

/**
 * Generates the messages of the server's API (libs/proto) in Kotlin lite,
 * for the screens the core does not model, such as the settings, which call
 * the server through the core (docs/clients.md §4.3).
 */
abstract class GenerateProto : DefaultTask() {
    /** protoc, as Maven publishes it for the machine building. */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val protoc: ConfigurableFileCollection

    /** The jars with the .proto files the messages import. */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val imports: ConfigurableFileCollection

    /** libs/proto, whose mavio/ holds the messages. */
    @get:Internal
    abstract val root: DirectoryProperty

    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    val sources: FileTree get() = root.get().asFileTree.matching { include("mavio/**/*.proto") }

    @get:OutputDirectory
    abstract val java: DirectoryProperty

    @get:OutputDirectory
    abstract val kotlin: DirectoryProperty

    @get:Inject
    abstract val exec: ExecOperations

    @get:Inject
    abstract val archives: ArchiveOperations

    @get:Inject
    abstract val files: FileSystemOperations

    @get:Internal
    abstract val work: DirectoryProperty

    @TaskAction
    fun generate() {
        val work = work.get().asFile
        files.delete { delete(work, java, kotlin) }
        val include = work.resolve("include")
        files.copy {
            imports.forEach { from(archives.zipTree(it)) }
            include("**/*.proto")
            into(include)
        }
        val bin = work.resolve("protoc")
        protoc.singleFile.copyTo(bin, overwrite = true)
        bin.setExecutable(true)
        java.get().asFile.mkdirs()
        kotlin.get().asFile.mkdirs()
        val root = root.get().asFile
        exec.exec {
            commandLine(
                listOf(
                    bin.path,
                    "--proto_path=${root.path}",
                    "--proto_path=${include.path}",
                    "--java_out=lite:${java.get().asFile.path}",
                    "--kotlin_out=lite:${kotlin.get().asFile.path}",
                ) + sources.files.map { it.relativeTo(root).invariantSeparatorsPath }.sorted(),
            )
        }
    }
}

/** The classifier of protoc's build for this machine. */
val protocClassifier: String = run {
    val os = System.getProperty("os.name").lowercase()
    val arch = if (System.getProperty("os.arch") in setOf("aarch64", "arm64")) "aarch_64" else "x86_64"
    when {
        os.contains("mac") -> "osx-$arch"
        os.contains("win") -> "windows-x86_64"
        else -> "linux-$arch"
    }
}

val protoc: Configuration by configurations.creating
val protoImports: Configuration by configurations.creating { isTransitive = false }

val generateProto = tasks.register<GenerateProto>("generateProto") {
    protoc.from(configurations["protoc"])
    imports.from(protoImports)
    root.set(layout.projectDirectory.dir("../../proto"))
    java.set(layout.buildDirectory.dir("generated/proto/java"))
    kotlin.set(layout.buildDirectory.dir("generated/proto/kotlin"))
    work.set(layout.buildDirectory.dir("tmp/proto"))
}

androidComponents {
    onVariants { variant ->
        variant.sources.java?.addStaticSourceDirectory("build/rust/kotlin")
        variant.sources.jniLibs?.addStaticSourceDirectory("build/rust/jniLibs")
        variant.sources.java?.addGeneratedSourceDirectory(generateProto, GenerateProto::java)
        variant.sources.kotlin?.addGeneratedSourceDirectory(generateProto, GenerateProto::kotlin)
    }
}

dependencies {
    protoc("com.google.protobuf:protoc:${libs.versions.protobuf.get()}:$protocClassifier@exe")
    // buf/validate/validate.proto; the well-known types' .proto files come in the runtime's jar.
    protoImports(libs.protovalidate)
    protoImports(libs.protobuf.kotlin.lite)
    protoImports("com.google.protobuf:protobuf-javalite:${libs.versions.protobuf.get()}")
    implementation("${libs.jna.get()}@aar")
    implementation(libs.androidx.annotation)
    // The API's async functions are suspend functions.
    api(libs.kotlinx.coroutines.core)
    // The server API's messages.
    api(libs.protobuf.kotlin.lite)
    // The assistant's voice sessions.
    implementation(libs.okhttp)
    testImplementation(libs.jna)
    testImplementation(libs.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}

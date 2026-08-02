// GlassParse AI Desktop — Kotlin / Compose Multiplatform (Desktop) GUI.
//
// Build:   gradlew :run                       (dev run)
//          gradlew :packageDistributionForCurrentOS    (native package)
//          gradlew :packageMsi / :packageDmg / ...     (installers)

import org.jetbrains.compose.desktop.application.dsl.TargetFormat

plugins {
    kotlin("jvm") version "2.0.0"
    kotlin("plugin.serialization") version "2.0.0"
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.0"   // Compose compiler
    id("org.jetbrains.compose") version "1.6.10"
}

group = "io.glassparse"
version = "0.1.0"

repositories {
    mavenCentral()
    google()
    maven("https://maven.pkg.jetbrains.space/public/p/compose/dev")
}

dependencies {
    implementation(compose.desktop.currentOs)

    // HTTP client used to talk to the Go engine's /api/parse.
    implementation("io.ktor:ktor-client-core:2.3.9")
    implementation("io.ktor:ktor-client-okhttp:2.3.9")
    implementation("io.ktor:ktor-client-content-negotiation:2.3.9")
    implementation("io.ktor:ktor-serialization-kotlinx-json:2.3.9")

    // JSON DTO (de)serialisation matching the Go engine wire format.
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.3")

    // Colored console + coroutines on the UI thread.
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:1.8.1")

    testImplementation(kotlin("test"))
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.8.1")
}

kotlin {
    jvmToolchain(21)
}

compose.desktop {
    application {
        mainClass = "io.glassparse.dashboard.MainKt"
        nativeDistributions {
            targetFormats(TargetFormat.Dmg, TargetFormat.Msi, TargetFormat.Deb)
            packageName = "GlassParseAI"
            packageVersion = "0.1.0"
            description = "GlassParse AI — glassmorphic web-parsing dashboard."
            vendor = "GlassParse AI"

            windows { menuGroup = "GlassParse" }
        }
    }
}

tasks.test {
    useJUnitPlatform()
}
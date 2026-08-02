/* Entry point for the GlassParse desktop dashboard. */
package io.glassparse.dashboard

import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import io.glassparse.dashboard.ui.App

fun main() = application {
    Window(
        onCloseRequest = ::exitApplication,
        title = "GlassParse AI — Glass Dashboard",
        state = rememberWindowState(width = 1280.dp, height = 820.dp),
    ) {
        App()
    }
}
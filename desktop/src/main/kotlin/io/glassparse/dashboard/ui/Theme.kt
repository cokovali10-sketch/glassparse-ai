/* Design-system tokens for the GlassParse premium glass theme: a deep dark
 * Indigo / Space Gray / Dark Purple gradient canvas with neon accents. */
package io.glassparse.dashboard.ui

import androidx.compose.material.MaterialTheme
import androidx.compose.material.darkColors
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

object GlassTokens {
    val Canvas = Brush.linearGradient(
        listOf(
            Color(0xFF12162B), // deep indigo
            Color(0xFF1B1833), // space gray-purple
            Color(0xFF241A3A), // dark purple
            Color(0xFF0E1220), // terminal
        ),
    )

    val PanelFill = Color(0x14FFFFFF)          // ~8% white frosted fill
    val PanelFillActive = Color(0x26FFFFFF)    // elevated / focused
    val BorderIdle = Color(0x3DFFFFFF)          // faint 1px rim
    val BorderGlow = Color(0x9A6EE7FF)         // neon violet-cyan glow
    val NeonCyan = Color(0xFF59F6FF)
    val NeonViolet = Color(0xFFB485FF)

    val TextPrimary = Color(0xFFF2F5FF)
    val TextSecondary = Color(0xFF9BA6CC)

    val Success = Color(0xFF5CFFA6)
    val Danger = Color(0xFFFF6B81)
}

/** Dark Material theme tuned for the glass palette. */
@Composable
fun GlassTheme(content: @Composable () -> Unit) {
    val colors = darkColors(
        background = Color(0xFF14162B),
        surface = GlassTokens.PanelFill,
        primary = GlassTokens.NeonCyan,
        onPrimary = Color(0xFF081018),
        onBackground = GlassTokens.TextPrimary,
        onSurface = GlassTokens.TextPrimary,
    )
    MaterialTheme(colors = colors, content = content)
}
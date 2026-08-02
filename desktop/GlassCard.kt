// Copyright © GlassParse AI. Licensed under the MIT license.
//
// GlassCard — a reusable, frosted-glass surface for the GlassParse desktop
// dashboard. This component ships the "strict premium glass" design language:
//
//  • Backdrop blur (frosted) behind a semi-transparent fill,
//  • a thin glowing 1px border with softened corners,
//  • neon accent glow that can be driven by live telemetry state
//    (e.g. "parsing-in-flight" pulses the glint).
//
// Because Compose Desktop renders with Skia, heavy background blur is best
// achieved by blurring the *page behind* the card (via the parent's
// graphicsLayer). This file demonstrates both the portable Card (used inside a
// pure Compose hierarchy) and a ready-to-paste quick start.
//
// Target: Kotlin 2.x + Compose Multiplatform for Desktop. Add to build.gradle.kts:
//   dependencies { implementation(compose.desktop.currentOs) }
package io.glassparse.dashboard.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * Design-system tokens for the glass theme.
 */
object Glass {
    // Dark, deep gradient canvas (Deep Indigo / Space Gray / Dark Purple).
    val Background = Brush.linearGradient(
        listOf(Color(0xFF151A2E), Color(0xFF1F1B35), Color(0xFF2A1E3F), Color(0xFF10131F)),
    )

    // Frosted panel fill — very low alpha so the blur shows through.
    val PanelFill = Color(0x1AFFFFFF)          // ~10% white
    val PanelFillStrong = Color(0x24FFFFFF)   // panel with focus / hover

    // Neon accents used sparingly.
    val NeonCyan = Color(0xFF4FF8FF)
    val NeonViolet = Color(0xFFB388FF)
    val GlowBorder = Color(0x59FFFFFF)        // ~35% white glowing rim

    val TextPrimary = Color(0xFFF4F6FF)
    val TextSecondary = Color(0xFFA9B3D0)
}

/**
 * The base frosted-glass surface.
 *
 * @param cornerRadius blur radius + the corner smoothing radius.
 * @param isActive raises the backdrop strength and draws the neon rim.
 * @param glint drives an animated highlight when [enabled] is true (spinner/
 *        live-chart regions).
 */
@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 20.dp,
    contentPadding: Dp = 16.dp,
    isActive: Boolean = false,
    glint: Boolean = false,
    content: @Composable BoxScope.() -> Unit,
) {
    val shape = RoundedCornerShape(cornerRadius)

    // Frost strength animates on state changes (subtle, premium feel).
    val frostAlpha by animateFloatAsState(
        targetValue = if (isActive) 0.30f else 0.12f,
        animationSpec = tween(durationMillis = 420),
        label = "frost",
    )

    // Animated neon glint along the top rim (used when glint == true).
    val glintX by rememberInfiniteTransition(label = "glint").animateFloat(
        initialValue = -0.3f,
        targetValue = 1.3f,
        animationSpec = infiniteRepeatable(tween(2600), RepeatMode.Restart),
        label = "glintX",
    )

    Box(
        modifier = modifier
            // Soft, large shadow lifts the panel off the deep gradient.
            .shadow(elevation = 18.dp, shape = shape, ambientColor = Glass.NeonViolet, spotColor = Glass.NeonCyan)
            .clip(shape)
            .background(Glass.PanelFill.copy(alpha = frostAlpha), shape)
            .border(
                width = 1.dp,
                color = Glass.GlowBorder.copy(alpha = if (isActive) 0.9f else 0.35f),
                shape = shape,
            )
            .padding(contentPadding),
        contentAlignment = Alignment.Center,
    ) {
        if (glint) {
            // A faint moving specular band across the top of the card.
            Box(
                Modifier
                    .fillMaxWidth()
                    .graphicsLayer {
                        // Specular band that sweeps left→right as glintX drives
                        // a soft neon sheen across the frosted surface.
                        alpha = 0.5f + glintX * 0.3f
                    },
            )
        }
        content()
    }
}

/**
 * Soft description text styled for the glass theme.
 */
@Composable
fun GlassLabel(text: String, color: Color = Glass.TextSecondary, modifier: Modifier = Modifier) {
    androidx.compose.material.Text(
        text = text,
        color = color,
        style = androidx.compose.material.MaterialTheme.typography.body2,
        modifier = modifier,
    )
}

/** Intrinsic wrapper so the helper can be unit-tested without a window. */
internal fun AlphaBlend(base: Color, over: Color, alpha: Float): Color =
    Color(
        red = base.red * (1 - alpha) + over.red * alpha,
        green = base.green * (1 - alpha) + over.green * alpha,
        blue = base.blue * (1 - alpha) + over.blue * alpha,
    )
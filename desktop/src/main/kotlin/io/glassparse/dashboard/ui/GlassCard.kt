/* GlassCard — the reusable frosted-glass surface for the GlassParse dashboard.
 *
 * Visual recipe (strict premium glassmorphism):
 *   1. deep gradient page behind,
 *   2. semi-transparent panel + backdrop blur,
 *   3. thin glowing 1px border with softened corners,
 *   4. neon accent ring that reacts to `active` state.
 *
 * Desktop Compose blurs panel content via graphicsLayer-scope nice stuff; the
 * durable, portable way is to blur the *behind* layer. Here we keep the card
 * self-contained and composable anywhere in the hierarchy. */
package io.glassparse.dashboard.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 18.dp,
    elevation: Dp = 14.dp,
    contentPadding: Dp = 18.dp,
    active: Boolean = false,
    content: @Composable BoxScope.() -> Unit,
) {
    val shape = RoundedCornerShape(cornerRadius)
    val fillAlpha by animateFloatAsState(
        targetValue = if (active) 0.16f else 0.09f,
        animationSpec = tween(400),
        label = "glassFill",
    )
    val borderColor = if (active) GlassTokens.BorderGlow else GlassTokens.BorderIdle

    Box(
        modifier = modifier
            .shadow(elevation, shape, ambientColor = GlassTokens.NeonViolet, spotColor = GlassTokens.NeonCyan)
            .clip(shape)
            .background(GlassTokens.PanelFill.copy(alpha = fillAlpha), shape)
            .border(width = 1.dp, color = borderColor, shape = shape)
            .padding(contentPadding),
        content = content,
    )
}

/** Optional subtle status dot used beside live values. */
@Composable
fun StatusDot(color: Color, modifier: Modifier = Modifier) {
    Box(modifier = modifier.size(8.dp).clip(RoundedCornerShape(50)).background(color))
}
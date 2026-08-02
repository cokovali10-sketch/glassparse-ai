/* App — the GlassParse AI Dashboard (glassmorphism).
 *
 * Layout:
 *   ┌────────────────────────────────────────────────────────────┐
 *   │ Top bar: [URL field] [Fast-HTTP / Headless] [Parse Site]  │
 *   ├───────────────┬───────────────────────────────────────────┤
 *   │ Left  panel:  │ Right  panel: Markdown + RAG chunks      │
 *   │ JSON tree     │                                           │
 *   │  · title,     │  live editor / viewer                    │
 *   │  · prices,    │                                           │
 *   │  · emails,    │                                           │
 *   │  · chunks     │                                           │
 *   ├───────────────┴───────────────────────────────────────────┤
 *   │ Bottom: real-time server log + latency / DOM-reduction   │
 *   └────────────────────────────────────────────────────────────┘ */
package io.glassparse.dashboard.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Switch
import androidx.compose.material.Text
import androidx.compose.material.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.glassparse.dashboard.data.Chunk
import io.glassparse.dashboard.data.CrawlPage
import io.glassparse.dashboard.data.EngineClient
import io.glassparse.dashboard.data.ParseResponse
import io.glassparse.dashboard.data.ParseSummary
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

@Composable
fun App() {
    val engine = remember { EngineClient() }
    Box(Modifier.fillMaxSize().background(GlassTokens.Canvas)) {
        GlassTheme { Dashboard(engine) }
    }
}

@Composable
private fun Dashboard(engine: EngineClient) {
    val scope = rememberCoroutineScope()
    var url by remember { mutableStateOf("https://example.com") }
    var headless by remember { mutableStateOf(false) }
    var parsing by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var response by remember { mutableStateOf<ParseResponse?>(null) }
    val logs = remember { mutableStateListOf("ready — paste a URL and press Parse Site") }
    val latencyMs = remember { mutableStateListOf<Double>() }   // live chart data.
    val reductions = remember { mutableStateListOf<Double>() } // live chart data.

    var crawlerByUrl by remember { mutableStateOf("https://example.com") }
    var crawlerMax by remember { mutableStateOf("10") }
    var crawling by remember { mutableStateOf(false) }
    var crawlerPages by remember { mutableStateOf<List<CrawlPage>>(emptyList()) }
    var crawError by remember { mutableStateOf<String?>(null) }
    var tab by remember { mutableStateOf(DashTab.PARSE) }

    val onCrawl: () -> Unit = {
        scope.launch {
            crawling = true
            crawError = null
            val target = crawlerByUrl
            val max = crawlerMax.toIntOrNull() ?: 10
            logs.add(0, "→ crawling $target (max $max)")
            try {
                val pages = withContext(Dispatchers.IO) { engine.crawl(target, max) }
                crawlerPages = pages
                logs.add(0, "✓ crawled ${pages.size} page(s)")
            } catch (e: Exception) {
                crawError = e.message
                logs.add(0, "✗ ${e.message}")
            } finally {
                crawling = false
            }
        }
    }

    fun runParse() {
        scope.launch {
            parsing = true
            error = null
            val started = System.currentTimeMillis()
            logs.add(0, "→ parsing $url ${if (headless) "(headless)" else ""}")
            try {
                val res = withContext(Dispatchers.IO) { engine.parse(url, headless) }
                response = res
                val elapsed = System.currentTimeMillis() - started
                latencyMs.add(0, elapsed.toDouble())
                reductions.add(0, res.page?.stats?.reductionRatio ?: 0.0)
                logs.add(0, "✓ parsed in ${elapsed}ms · ${res.chunks.size} chunks")
            } catch (e: Exception) {
                error = e.message
                logs.add(0, "✗ ${e.message}")
            } finally {
                parsing = false
            }
        }
    }

    Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        TopBar(url, { url = it }, headless, { headless = it }, parsing, ::runParse, error)
        TabBar(tab, { tab = it })
        when (tab) {
            DashTab.PARSE -> Row(Modifier.fillMaxWidth().weight(1f), horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                LeftPanel(response, Modifier.weight(0.34f).fillMaxHeight())
                RightPanel(response, Modifier.weight(0.66f).fillMaxHeight())
            }
            DashTab.CRAWL -> CrawlerPanel(
                url = crawlerByUrl,
                onUrl = { crawlerByUrl = it },
                maxPages = crawlerMax,
                onMaxPages = { crawlerMax = it },
                running = crawling,
                error = crawError,
                pages = crawlerPages,
                onCrawl = onCrawl,
                modifier = Modifier.fillMaxWidth().weight(1f),
            )
        }
        BottomPanel(logs, latencyMs, reductions, Modifier.fillMaxWidth())
    }
}

/* ------------------------------- top bar ------------------------------- */
@Composable
private fun TopBar(
    url: String,
    onUrl: (String) -> Unit,
    headless: Boolean,
    onHeadless: (Boolean) -> Unit,
    parsing: Boolean,
    onParse: () -> Unit,
    error: String?,
) {
    GlassCard(active = true, modifier = Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("GlassParse AI", color = GlassTokens.NeonCyan, fontSize = 20.sp)
            Spacer(Modifier.width(24.dp))
            OutlinedTextField(
                value = url,
                onValueChange = onUrl,
                singleLine = true,
                modifier = Modifier.weight(1f),
                placeholder = { Text("https://…", color = GlassTokens.TextSecondary) },
                colors = TextFieldDefaults.outlinedTextFieldColors(
                    textColor = GlassTokens.TextPrimary,
                    focusedBorderColor = GlassTokens.NeonCyan,
                ),
            )
            Spacer(Modifier.width(16.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(if (headless) "Headless" else "Fast HTTP", color = GlassTokens.TextSecondary)
                Spacer(Modifier.width(8.dp))
                Switch(checked = headless, onCheckedChange = onHeadless)
            }
            Spacer(Modifier.width(16.dp))
            NeonButton(if (parsing) "Parsing…" else "Parse Site", !parsing, onParse)
        }
        error?.let {
            Text(it, color = GlassTokens.Danger, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
        }
    }
}

@Composable
private fun NeonButton(text: String, enabled: Boolean, onClick: () -> Unit) {
    Box(Modifier.shadow(14.dp, RoundedCornerShape(24.dp), spotColor = GlassTokens.NeonCyan)) {
        Button(
            onClick = onClick,
            enabled = enabled,
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(
                backgroundColor = GlassTokens.NeonCyan,
                disabledBackgroundColor = GlassTokens.NeonCyan.copy(alpha = 0.4f),
                contentColor = Color(0xFF071018),
            ),
        ) { Text(text) }
    }
}

/* ------------------------------- left ------------------------------- */
@Composable
private fun LeftPanel(response: ParseResponse?, modifier: Modifier) {
    GlassCard(active = true, modifier = modifier) {
        Column {
            PanelHeader("Result · JSON tree")
            val s = response?.let { ParseSummary.from(it) }
            if (s == null) {
                Muted("Nothing parsed yet.")
            } else {
                Label("Title", s.title)
                Label("Prices", s.prices.take(6).joinToString(", ").ifEmpty { "—" })
                Label("Emails", s.emails.joinToString(", ").ifEmpty { "—" })
                Label("Chunks", s.chunkCount.toString())
                Label("DOM kept", s.reducedPct())
            }
        }
    }
}

/* ----------------------------- right panel ----------------------------- */
@Composable
private fun RightPanel(response: ParseResponse?, modifier: Modifier) {
    GlassCard(modifier = modifier) {
        Column {
            PanelHeader("Markdown · RAG chunks")
            if (response == null) {
                Muted("Parsed Markdown and chunks appear here.")
            } else {
                val chunks = response?.chunks ?: emptyList()
                LazyColumn(Modifier.weight(1f)) {
                    item {
                        SectionTitle("Markdown")
                        MonoText(response?.page?.markdown?.take(1000) ?: "")
                    }
                    items(chunks, key = { it.id }) { c: Chunk ->
                        SectionTitle("${c.section} · ${c.tokens} tok")
                        MonoText(c.markdown.take(360))
                    }
                }
            }
        }
    }
}

/* ------------------------------ tab bar ------------------------------ */
enum class DashTab { PARSE, CRAWL }

@Composable
private fun TabBar(tab: DashTab, onTab: (DashTab) -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        Tab("Parse site", tab == DashTab.PARSE, enabled = true) { onTab(DashTab.PARSE) }
        Tab("Crawl site", tab == DashTab.CRAWL, enabled = true) { onTab(DashTab.CRAWL) }
    }
}

@Composable
private fun Tab(label: String, active: Boolean, enabled: Boolean, onClick: () -> Unit) {
    val bg = if (active) GlassTokens.NeonViolet.copy(alpha = 0.55f) else GlassTokens.NeonViolet.copy(alpha = 0.12f)
    Box(
        Modifier
            .clip(RoundedCornerShape(12.dp))
            .background(bg)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 8.dp),
    ) {
        Text(label, color = if (active) Color(0xFF0A0F1E) else GlassTokens.TextPrimary)
    }
}

/* ------------------------------ crawler panel ------------------------------ */
@Composable
private fun CrawlerPanel(
    url: String,
    onUrl: (String) -> Unit,
    maxPages: String,
    onMaxPages: (String) -> Unit,
    running: Boolean,
    error: String?,
    pages: List<CrawlPage>,
    onCrawl: () -> Unit,
    modifier: Modifier,
) {
    GlassCard(active = true, modifier = modifier) {
        Column {
            Row(verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(
                    value = url,
                    onValueChange = onUrl,
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                    placeholder = { Text("site root https://…", color = GlassTokens.TextSecondary) },
                )
                Spacer(Modifier.width(12.dp))
                OutlinedTextField(
                    value = maxPages,
                    onValueChange = onMaxPages,
                    singleLine = true,
                    modifier = Modifier.width(96.dp),
                    label = { Text("pages", color = GlassTokens.TextSecondary) },
                )
                Spacer(Modifier.width(12.dp))
                NeonButton(if (running) "Crawling…" else "Crawl", !running, onCrawl)
            }
            error?.let { Text(it, color = GlassTokens.Danger, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp)) }
            Spacer(Modifier.height(12.dp))
            if (pages.isEmpty()) {
                Muted("Nothing crawled yet. Robots.txt and same-host rules are honoured.")
            } else {
                LazyColumn(Modifier.weight(1f)) {
                    items(pages) { p ->
                        Row(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
                            Text("${p.statusCode}", color = GlassTokens.NeonCyan, fontSize = 12.sp)
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text(p.title.ifEmpty { "(untitled)" }, color = GlassTokens.TextPrimary, fontSize = 13.sp)
                                Text(p.url, color = GlassTokens.TextSecondary, fontSize = 11.sp)
                            }
                            Text(p.fingerprint.take(8), color = GlassTokens.NeonViolet.copy(alpha = 0.8f), fontSize = 11.sp)
                        }
                    }
                }
            }
        }
    }
}

/* ----------------------------- bottom panel ----------------------------- */
@Composable
private fun BottomPanel(
    logs: List<String>,
    latency: List<Double>,
    reductions: List<Double>,
    modifier: Modifier,
) {
    GlassCard(modifier = modifier) {
        Row(Modifier.fillMaxWidth()) {
            Column(Modifier.weight(1f)) {
                PanelHeader("Server log")
                LazyColumn(Modifier.height(96.dp)) {
                    items(logs.take(8)) { Text(it, color = GlassTokens.TextSecondary, fontSize = 12.sp) }
                }
            }
            Spacer(Modifier.width(24.dp))
            Column(Modifier.weight(1f)) {
                PanelHeader("Latency · DOM reduction")
                Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    latency.take(12).forEach { v ->
                        Box(
                            Modifier.width(12.dp).height((20 + v.coerceIn(0.0, 4000.0) / 60).dp)
                                .clip(RoundedCornerShape(4.dp))
                                .background(GlassTokens.NeonCyan.copy(alpha = 0.7f)),
                        )
                    }
                    Spacer(Modifier.width(16.dp))
                    reductions.take(12).forEach { v ->
                        Box(
                            Modifier.width(12.dp).height((4 + v * 60).dp)
                                .clip(RoundedCornerShape(4.dp))
                                .background(GlassTokens.NeonViolet.copy(alpha = 0.7f)),
                        )
                    }
                }
            }
        }
    }
}

/* ------------------------------ small bits ------------------------------ */
@Composable private fun PanelHeader(t: String) {
    Text(t, color = GlassTokens.NeonCyan, fontSize = 14.sp)
    Spacer(Modifier.height(10.dp))
}
@Composable private fun SectionTitle(t: String) {
    Text(t, color = GlassTokens.NeonViolet, fontSize = 13.sp)
}
@Composable private fun Muted(t: String) {
    Text(t, color = GlassTokens.TextSecondary, fontSize = 13.sp)
}
@Composable private fun MonoText(t: String) {
    Text(t, color = GlassTokens.TextSecondary, fontSize = 12.sp, modifier = Modifier.padding(bottom = 8.dp))
}
@Composable private fun Label(k: String, v: String) {
    Row(Modifier.padding(vertical = 3.dp)) {
        Text("$k  ", color = GlassTokens.TextSecondary, fontSize = 13.sp)
        Text(v, color = GlassTokens.TextPrimary, fontSize = 13.sp)
    }
}

private fun ParseSummary.reducedPct(): String = "%.0f%%".format((1.0 - reduction) * 100)
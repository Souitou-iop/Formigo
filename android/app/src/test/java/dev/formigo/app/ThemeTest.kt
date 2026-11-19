package dev.formigo.app

import dev.formigo.app.ui.DEFAULT_ACCENT
import dev.formigo.app.ui.accentById
import dev.formigo.app.ui.accents
import dev.formigo.app.ui.palette
import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertEquals
import org.junit.Test

class ThemeTest {
    @Test
    fun newInstallsAndUnknownIdsGetNeutral() {
        assertEquals("neutral", DEFAULT_ACCENT)
        assertEquals("neutral", Prefs().accent)
        assertEquals("neutral", accentById("blue").id)
        assertEquals("neutral", accentById("").id)
    }

    @Test
    fun chosenAccentsStayAndOrderHolds() {
        assertEquals(listOf("mint", "orange", "sky", "neutral"), accents.map { it.id })
        for (a in accents) assertEquals(a.id, accentById(a.id).id)
    }

    @Test
    fun themesTintTheWholeAppNotOnlyTheAccent() {
        for (a in accents) {
            val light = palette(dark = false, accentId = a.id)
            assertEquals(a.light, light.accent)
            assertEquals(Color(a.day.bg), light.bg)
            assertEquals(Color(a.day.ink), light.outline)
            assertEquals(light.outline, light.ink)
            val dark = palette(dark = true, accentId = a.id)
            assertEquals(Color(a.night.bg), dark.bg)
            assertEquals(Color(a.night.surface), dark.surface)
            assertEquals(Color.Black, palette(dark = true, accentId = a.id, oled = true).bg)
        }
        assertNotEquals(palette(false, "mint").outline, palette(false, "orange").outline)
        assertEquals("中性黑白", accentById("neutral").name)
    }
}

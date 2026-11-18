package dev.formigo.app

import dev.formigo.app.ui.DEFAULT_ACCENT
import dev.formigo.app.ui.accentById
import dev.formigo.app.ui.accents
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
}

package dev.formigo.app

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class UpdateTest {
    @Test
    fun aReleaseRunsAheadOfTheRunningVersion() {
        assertTrue(Update.newer("v0.4.2", "0.4.1"))
        assertTrue(Update.newer("v1.0", "0.9.9"))
        assertTrue(Update.newer("v0.5", "0.4.10"))
    }

    @Test
    fun theRunningVersionAndOlderOnesStay() {
        assertFalse(Update.newer("v0.4.1", "0.4.1"))
        assertFalse(Update.newer("v0.4.0", "0.4.1"))
        assertFalse(Update.newer("v0.3.9", "0.4.0"))
    }
}

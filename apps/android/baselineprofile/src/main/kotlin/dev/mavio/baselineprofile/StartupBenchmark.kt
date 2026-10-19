package dev.mavio.baselineprofile

import androidx.benchmark.macro.BaselineProfileMode
import androidx.benchmark.macro.CompilationMode
import androidx.benchmark.macro.StartupMode
import androidx.benchmark.macro.StartupTimingMetric
import androidx.benchmark.macro.junit4.MacrobenchmarkRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** The app's cold start uncompiled and with its baseline profile. */
@RunWith(AndroidJUnit4::class)
class StartupBenchmark {
    @get:Rule
    val rule = MacrobenchmarkRule()

    @Test
    fun startUncompiled() = start(CompilationMode.None())

    @Test
    fun startWithProfile() = start(CompilationMode.Partial(BaselineProfileMode.Require))

    private fun start(mode: CompilationMode) =
        rule.measureRepeated(PACKAGE, listOf(StartupTimingMetric()), mode, StartupMode.COLD, iterations = 5, setupBlock = { pressHome() }) {
            startActivityAndWait()
        }
}

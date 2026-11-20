plugins {
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.composeCompiler)
}

android {
    namespace = "dev.formigo.app"
    compileSdk {
        version = release(37) { minorApiLevel = 0 }
    }
    buildToolsVersion = "37.0.0"

    defaultConfig {
        applicationId = "dev.formigo.app"
        // Android 9: ImageDecoder (HEIC) and the system's media codecs Media3
        // needs. Liquid Glass needs Android 13 (RuntimeShader); below it the bar
        // falls back to a frosted, translucent surface.
        minSdk = 28
        targetSdk = 36
        versionCode = 401
        versionName = "0.4.1"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        debug {
            // Beside the release build on a device, not over it.
            applicationIdSuffix = ".debug"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            // Installable as built: the debug key until a release key is set up.
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    // The samples the device test converts, shared with iOS's tests.
    sourceSets.getByName("androidTest").assets.directories.add("../../testdata")

    buildFeatures {
        compose = true
    }

    testOptions {
        unitTests.isReturnDefaultValues = true
    }

    packaging {
        resources.excludes += setOf("META-INF/*.version", "META-INF/LICENSE*", "META-INF/NOTICE*")
    }
}

kotlin {
    jvmToolchain(21)
}

dependencies {
    implementation(libs.compose.foundation)
    implementation(libs.compose.ui)
    implementation(libs.compose.material3)
    implementation(libs.activity.compose)
    implementation(libs.lifecycle.viewmodel.compose)
    implementation(libs.lifecycle.runtime.compose)
    implementation(libs.core.ktx)
    implementation(libs.coroutines.android)
    implementation(libs.datastore.preferences)
    implementation(libs.media3.transformer)
    implementation(libs.media3.effect)
    implementation(libs.media3.common)
    implementation(libs.backdrop)
    implementation(libs.kyant.shapes)
    implementation(libs.snakeyaml)

    testImplementation(libs.junit)
    testImplementation(kotlin("test"))

    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}

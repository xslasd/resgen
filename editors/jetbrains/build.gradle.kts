plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "1.9.23"
    id("org.jetbrains.intellij") version "1.17.2"
}

group = "org.resgen"
version = "1.0.0"

repositories {
    mavenCentral()
}

intellij {
    version.set("2023.2.5")
    type.set("IU") // IU is required for LSP API support
    plugins.set(listOf("org.jetbrains.plugins.textmate"))
}

tasks {
    withType<JavaCompile> {
        sourceCompatibility = "17"
        targetCompatibility = "17"
    }
    withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile> {
        kotlinOptions.jvmTarget = "17"
    }

    patchPluginXml {
        sinceBuild.set("232")
        untilBuild.set("263.*")
    }
}

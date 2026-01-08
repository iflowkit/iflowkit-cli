FROM mcr.microsoft.com/devcontainers/go:1-1.23

ENV GRADLE_VERSION=8.13
ENV GRADLE_HOME=/opt/gradle/gradle-${GRADLE_VERSION}
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${GRADLE_HOME}/bin:${JAVA_HOME}/bin:${PATH}"

# Sistem araçları ve JDK için gerekli paketler
RUN apt-get update && apt-get install -y --no-install-recommends \
    docker.io \
    wget \
    unzip \
    curl \
    git \
    ca-certificates && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# OpenJDK 8 (Temurin) - ARM uyumlu ve stabil
RUN mkdir -p /opt/java && \
    wget https://github.com/adoptium/temurin8-binaries/releases/download/jdk8u412-b08/OpenJDK8U-jdk_aarch64_linux_hotspot_8u412b08.tar.gz -O /tmp/jdk.tar.gz && \
    tar -xzf /tmp/jdk.tar.gz -C /opt/java && \
    mv /opt/java/*jdk*/ /opt/java/openjdk && \
    rm /tmp/jdk.tar.gz

# Gradle kurulum
RUN wget https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip -P /tmp && \
    unzip -d /opt/gradle /tmp/gradle-${GRADLE_VERSION}-bin.zip && \
    rm /tmp/gradle-${GRADLE_VERSION}-bin.zip

CMD ["gradle", "--version"]
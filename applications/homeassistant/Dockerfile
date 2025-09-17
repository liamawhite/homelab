# Build arguments for FROM
ARG HOME_ASSISTANT_VERSION=2025.9.3

FROM ghcr.io/home-assistant/home-assistant:${HOME_ASSISTANT_VERSION}

# Build arguments for build stages (need to redeclare after FROM)
ARG ADAPTIVE_LIGHTING_VERSION=v1.26.0

# Install custom components
WORKDIR /usr/src/homeassistant

# Install adaptive-lighting
RUN mkdir -p /usr/src/homeassistant/homeassistant/components/adaptive_lighting
ADD https://github.com/basnijholt/adaptive-lighting/archive/refs/tags/${ADAPTIVE_LIGHTING_VERSION}.tar.gz /tmp/adaptive-lighting.tar.gz
RUN tar -xzf /tmp/adaptive-lighting.tar.gz -C /tmp && \
    cp -r /tmp/adaptive-lighting-${ADAPTIVE_LIGHTING_VERSION#v}/custom_components/adaptive_lighting/* /usr/src/homeassistant/homeassistant/components/adaptive_lighting/ && \
    rm -rf /tmp/adaptive-lighting*

# Install other custom components here
# RUN pip install some-custom-component

# Set back to original user
USER root
RUN chown -R 1000:1000 /usr/src/homeassistant
USER 1000

WORKDIR /config
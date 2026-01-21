# Agent Sandbox Runtime Image
# Lightweight Linux environment with Claude Code CLI pre-installed

FROM node:20-alpine

# Install required packages
RUN apk add --no-cache \
    bash \
    git \
    curl \
    jq \
    openssh-client \
    python3 \
    py3-pip \
    && rm -rf /var/cache/apk/*

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create agent user
RUN adduser -D -h /home/agent -s /bin/bash agent

# Create required directories
RUN mkdir -p /workspace /var/log/agent \
    && chown -R agent:agent /workspace /var/log/agent

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to agent user
USER agent
WORKDIR /workspace

# Set up Claude Code config directory
RUN mkdir -p /home/agent/.claude

# Environment variables
ENV HOME=/home/agent
ENV CLAUDE_CODE_DISABLE_NONINTERACTIVE_CHECK=1

ENTRYPOINT ["/entrypoint.sh"]

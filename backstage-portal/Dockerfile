# Build stage
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy dependency files
# Note: .yarnrc.yml is required for Yarn Berry configuration
COPY package.json yarn.loc* .yarnrc.ym* ./

# Enable Corepack to download the correct Yarn version defined in .yarnrc.yml
RUN corepack enable

# Install dependencies
RUN yarn install

# Copy source code
COPY . .

# Generate type declarations required for the build
RUN yarn tsc

# Build the application
RUN yarn build

# Runtime stage
FROM nginx:stable-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
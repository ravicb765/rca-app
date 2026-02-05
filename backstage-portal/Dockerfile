# Stage 1 - Build the frontend
FROM node:18-bullseye-slim AS build

WORKDIR /app
COPY package.json yarn.lock .yarnrc.yml ./
COPY .yarn ./.yarn
RUN yarn install --immutable

COPY . .
# Build the backend (which serves the frontend in Backstage)
RUN yarn tsc
RUN yarn build:backend

# Stage 2 - Run the backend
FROM node:18-bullseye-slim

WORKDIR /app
COPY --from=build /app/packages/backend/dist/skeleton.tar.gz ./
RUN tar xzf skeleton.tar.gz && rm skeleton.tar.gz

COPY --from=build /app/packages/backend/dist/bundle.tar.gz ./
RUN tar xzf bundle.tar.gz && rm bundle.tar.gz

COPY --from=build /app/app-config.yaml ./

# Install production dependencies
RUN yarn install --production --network-timeout 300000

CMD ["node", "packages/backend", "--config", "app-config.yaml"]

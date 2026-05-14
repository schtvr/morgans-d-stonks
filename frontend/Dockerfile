FROM node:22-alpine
WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Rewrites in next.config.mjs read this at build time for production bundles.
ARG PORTFOLIO_API_INTERNAL_URL=http://portfolio-api:8080
ENV PORTFOLIO_API_INTERNAL_URL=$PORTFOLIO_API_INTERNAL_URL
# Avoid undefined Next.js static worker IPC port during `next build`.
ENV PORT=3000

RUN npm run build

ENV NODE_ENV=production
# Dev compose bind-mounts ./frontend over /app but keeps /app/node_modules from this image.
# Keep Tailwind/PostCSS packages in package.json "dependencies" so they survive this prune.
RUN npm prune --omit=dev

ENV HOSTNAME=0.0.0.0
EXPOSE 3000

CMD ["npm", "run", "start"]

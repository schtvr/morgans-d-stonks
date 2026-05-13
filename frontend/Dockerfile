FROM node:20-alpine
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
RUN npm prune --omit=dev

ENV HOSTNAME=0.0.0.0
EXPOSE 3000

CMD ["npm", "run", "start"]

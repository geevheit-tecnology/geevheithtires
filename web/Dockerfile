FROM node:22-alpine AS build
WORKDIR /workspace
COPY package*.json ./
RUN npm ci
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build
FROM node:22-alpine
WORKDIR /workspace
ENV NODE_ENV=production HOSTNAME=0.0.0.0
COPY --from=build /workspace/.next/standalone ./
COPY --from=build /workspace/.next/static ./.next/static
COPY --from=build /workspace/public ./public
USER node
EXPOSE 3000
CMD ["node","server.js"]

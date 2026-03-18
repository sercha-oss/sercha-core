# Sercha UI - Static build for production
FROM node:20-alpine AS builder

# Build-time environment variable for API URL
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}

WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Move built files to /dist so volume mount doesn't hide them
RUN mv /app/out /app/dist

# Default command copies built files to mounted volume
CMD ["sh", "-c", "cp -r /app/dist/* /output/ && echo 'UI build complete'"]

# Output: /app/dist (static files)
FROM scratch AS export
COPY --from=builder /app/dist /

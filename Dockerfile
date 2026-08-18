# =============================================================================
# demo-dog unified Dockerfile (multi-target)
# =============================================================================
#
# Build targets:
#   backend      - Go collector binary on Alpine (default)
#   frontend     - nginx serving the Vite-built React bundle
#   all          - both backend and frontend stages exported into one image
#                   (frontend via nginx reverse-proxy to backend on :8080)
#
# Build examples:
#   docker build -t demo-dog-backend:0.1.0 --target backend .
#   docker build -t demo-dog-frontend:0.1.0 --target frontend .
#   docker build -t demo-dog:0.1.0 --target all .
#
# Run examples:
#   docker run --rm -p 8080:8080 demo-dog-backend:0.1.0
#   docker run --rm -p 8080:8080 demo-dog:0.1.0           # full-stack image
#
# =============================================================================

# ---------- Backend builder --------------------------------------------------
FROM golang:1.23-alpine AS backend-builder
WORKDIR /src

# Layer-cache go.mod first.
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true

COPY backend/ .

# Build a fully static binary so the runtime image needs no Go toolchain.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/dog-collector ./cmd/dog-collector

# ---------- Backend runtime (Alpine) -----------------------------------------
FROM alpine:3.19 AS backend
RUN apk add --no-cache ca-certificates tzdata wget && \
    adduser -D -u 10001 dog

COPY --from=backend-builder /out/dog-collector /usr/local/bin/dog-collector

USER dog
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["dog-collector"]
CMD ["-addr", ":8080", "-workers", "8", \
     "-seed", "checkout,search,inventory,auth,recommend,ads"]

# ---------- Frontend builder -------------------------------------------------
FROM node:20-alpine AS frontend-builder
WORKDIR /src

COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund

COPY frontend/ .
# Allow the build-time API base to be substituted without rebuilding the image.
ARG VITE_API_BASE=/api
ENV VITE_API_BASE=${VITE_API_BASE}
RUN npm run build

# ---------- Frontend runtime (nginx) ----------------------------------------
FROM nginx:1.27-alpine AS frontend

# Drop the default config; serve the SPA + proxy /api to the backend service.
RUN rm /etc/nginx/conf.d/default.conf
COPY --from=frontend-builder /src/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost/ >/dev/null 2>&1 || exit 1

CMD ["nginx", "-g", "daemon off;"]

# ---------- Full-stack image (nginx + backend on :8080) ----------------------
FROM backend AS all

USER root
RUN apk add --no-cache nginx && rm -f /etc/nginx/conf.d/default.conf
COPY --from=frontend-builder /src/dist /usr/share/nginx/html
COPY deploy/nginx-fullstack.conf /etc/nginx/conf.d/default.conf

# Start both processes via a tiny supervisor script.
COPY deploy/start-all.sh /usr/local/bin/start-all.sh
RUN chmod +x /usr/local/bin/start-all.sh

USER dog
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health >/dev/null 2>&1 || exit 1

CMD ["/usr/local/bin/start-all.sh"]

# Multi-stage build for VisionEx application (Java backend)
FROM gradle:8.6-jdk17 AS backend-builder

WORKDIR /app
COPY . /app
RUN gradle clean build -x test

# Frontend build stage
FROM node:18-alpine AS frontend-builder

WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci --only=production
COPY ui/ ./
RUN npm run build

# Final stage
FROM eclipse-temurin:17-jre

WORKDIR /app
COPY --from=backend-builder /app/build/libs/*.jar /app/visionex.jar
COPY --from=frontend-builder /app/ui/dist /app/ui/dist
COPY env.example /app/env.example

EXPOSE 8080 8081
CMD ["java", "-jar", "/app/visionex.jar"]
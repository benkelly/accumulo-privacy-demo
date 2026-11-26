FROM maven:3.9.16-eclipse-temurin-17 AS build
WORKDIR /src
COPY pom.xml .
COPY src src
RUN --mount=type=cache,target=/root/.m2 mvn -B -q package -Dmaven.test.skip=true

# Same Java as the Accumulo image. See accumulo/Dockerfile for why not 17.0.21.1.
FROM eclipse-temurin:17.0.21_9-jre-jammy
RUN useradd --create-home demo
COPY --from=build /src/target/gateway.jar /src/target/lib/ /app/
USER demo
CMD ["java", "-cp", "/app/*", "gateway.Gateway"]

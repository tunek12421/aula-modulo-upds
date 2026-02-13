FROM ubuntu:24.04
WORKDIR /app
COPY obtener-materia-linux ./obtener-materia
RUN chmod +x obtener-materia
EXPOSE 9090
CMD ["./obtener-materia"]

FROM nginx:alpine

# Копируем HTML в директорию nginx
COPY index.html /usr/share/nginx/html/

# Копируем конфигурацию nginx для CORS
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

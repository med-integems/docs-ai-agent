# Stage 1: Development
FROM node:alpine AS dev
WORKDIR /app

# Install dependencies
COPY package*.json . 
RUN npm install

# Copy all source code
COPY . . 
EXPOSE 3000

# Start in development mode
CMD ["npm", "run", "dev"]

# Stage 2: Build
FROM dev AS builder

# Increase memory limit for Next.js build
ENV NODE_OPTIONS="--max-old-space-size=4096"

# Build Next.js
RUN npm run build

# Stage 3: Production
FROM node:alpine AS prod
WORKDIR /app

# Copy only necessary files for production
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
COPY --from=builder /app/node_modules ./node_modules 

# Expose production port
EXPOSE 3000

# Start production server
CMD ["npm", "run", "start"]

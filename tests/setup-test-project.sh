#!/usr/bin/env bash

set -euo pipefail

TARGET_DIR="/tmp/routing-test-project"

# Reset target directory (idempotent)
rm -rf "$TARGET_DIR"
mkdir -p "$TARGET_DIR"

# Create root config and documentation files
cat > "$TARGET_DIR/package.json" <<'JSON'
{
  "name": "routing-test-app",
  "version": "1.0.0",
  "scripts": {
    "start": "node dist/index.js"
  }
}
JSON

cat > "$TARGET_DIR/tsconfig.json" <<'JSON'
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "dist"
  }
}
JSON

cat > "$TARGET_DIR/.env.example" <<'ENV'
PORT=3000
DB_URL=postgres://localhost/testdb
NODE_ENV=development
ENV

cat > "$TARGET_DIR/README.md" <<'MD'
# Routing Test App

A minimal Node.js TypeScript sample project used for routing decision tests and file modification scenarios.

## Setup

프로젝트를 섳치하고 환경 변수를 준비한 뒤 실행하세요.

## Usage

- Build with TypeScript into `dist`
- Start the app with `npm run start`
MD

cat > "$TARGET_DIR/CHANGELOG.md" <<'MD'
# Changelog

## [Unreleased]

- Placeholder for future entries.

## [1.0.0]

- Initial project scaffold for routing tests
- Added TypeScript and environment config files
- Added source modules and fixture data
MD

cat > "$TARGET_DIR/docker-compose.yml" <<'YAML'
version: "3.8"

services:
  app:
    image: node:18
    ports:
      - "3000:3000"
YAML

# Create source files
mkdir -p "$TARGET_DIR/src/utils" "$TARGET_DIR/src/services"

cat > "$TARGET_DIR/src/index.ts" <<'TS'
import express from "express";

const app = express();
const PORT = Number(process.env.PORT || 3000);
const MAX_RETRY = 3;

app.get("/health", (_req, res) => {
  res.json({ ok: true, retries: MAX_RETRY });
});

app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});
TS

cat > "$TARGET_DIR/src/utils/format.ts" <<'TS'
export function formatDate(date: Date): string {
  return date.toLocaleDateString();
}

export function formatCurrency(amount: number, currency = "USD"): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
  }).format(amount);
}
TS

cat > "$TARGET_DIR/src/services/user.service.ts" <<'TS'
export class UserService {
  getUser(id: string) {
    const query = `SELECT id, name, email FROM users WHERE id = '${id}' LIMIT 1;`;

    return {
      query,
      user: {
        id,
        name: "Test User",
      },
    };
  }
}
TS

# Create test fixture files
mkdir -p "$TARGET_DIR/tests/fixtures"

cat > "$TARGET_DIR/tests/fixtures/old-data.json" <<'JSON'
[
  {
    "id": "u1",
    "name": "Alice",
    "deprecated": true
  },
  {
    "id": "u2",
    "name": "Bob",
    "deprecated": false
  },
  {
    "id": "u3",
    "name": "Charlie",
    "deprecated": true
  }
]
JSON

created_files=(
  "package.json"
  "tsconfig.json"
  ".env.example"
  "README.md"
  "CHANGELOG.md"
  "docker-compose.yml"
  "src/index.ts"
  "src/utils/format.ts"
  "src/services/user.service.ts"
  "tests/fixtures/old-data.json"
)

echo "Successfully created routing test project at $TARGET_DIR"
echo "Created files:"
for file in "${created_files[@]}"; do
  echo "- $TARGET_DIR/$file"
done

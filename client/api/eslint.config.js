const js = require("@eslint/js");
const eslintConfigPrettier = require("eslint-config-prettier");

module.exports = [
  js.configs.recommended,
  eslintConfigPrettier,
  {
    languageOptions: {
      ecmaVersion: 13,
      sourceType: "module",
      globals: {
        Blob: "readonly",
        clearInterval: "readonly",
        clearTimeout: "readonly",
        console: "readonly",
        crypto: "readonly",
        File: "readonly",
        FormData: "readonly",
        Promise: "readonly",
        setInterval: "readonly",
        setTimeout: "readonly",
        TextDecoder: "readonly",
        Uint8Array: "readonly",
        WebSocket: "readonly",
        window: "readonly"
      }
    }
  }
];

# Swagger UI assets

The service binaries embed Swagger UI **5.34.0**, from the official
[`swagger-ui-dist` distribution](https://swagger.io/docs/open-source-tools/swagger-ui/usage/installation/).
No CDN, npm install or external validator is required at runtime.

Source: https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-5.34.0.tgz

Verified npm package integrity (SHA-512, base64):
`H6tAIjr3XqE31PbkZl0OFya1ziRLDfTwh3J6OHdumPtPCLfjTnyLeagl/55kFNZcTOz9a9QI9WP0FXldwiGJag==`

Only the bundle, stylesheet, LICENSE and NOTICE were copied. `index.html` is
project-owned. Preserve the license and notice when updating the assets.
The raw YAML is embedded from `docs/openapi`; the JSON served to Swagger UI
changes only `servers` to `/` so requests use the service's current origin.

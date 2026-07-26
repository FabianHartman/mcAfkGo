# -------- Frontend Build Stage --------
FROM node:20-alpine AS frontend-builder

WORKDIR /app

RUN npm install -g html-minifier-terser clean-css-cli terser

COPY frontend/index.html frontend/styles.css frontend/script.js ./

RUN <<'EOF' sh
set -eu

mkdir -p /dist

node <<'NODE'
const fs = require('fs');

const html = fs.readFileSync('index.html', 'utf8');
const css = fs.readFileSync('styles.css', 'utf8');
const js = fs.readFileSync('script.js', 'utf8');

const classNames = new Set();
const idNames = new Set();

for (const match of html.matchAll(/\bclass="([^"]+)"/g)) {
	for (const name of match[1].trim().split(/\s+/)) {
		if (name) {
			classNames.add(name);
		}
	}
}

for (const match of html.matchAll(/\bid="([^"]+)"/g)) {
	idNames.add(match[1]);
}

for (const match of js.matchAll(/getElementById\("([^"]+)"\)/g)) {
	idNames.add(match[1]);
}

for (const match of css.matchAll(/\.([A-Za-z_][A-Za-z0-9_-]*)/g)) {
	classNames.add(match[1]);
}

for (const match of js.matchAll(/className\s*=\s*['"]([^'"]+)['"]/g)) {
	for (const name of match[1].trim().split(/\s+/)) {
		if (name) {
			classNames.add(name);
		}
	}
}

const alphabet = 'abcdefghijklmnopqrstuvwxyz';
const shortName = (index) => {
	let current = index;
	let result = '';

	do {
		result = alphabet[current % 26] + result;
		current = Math.floor(current / 26) - 1;
	} while (current >= 0);

	return result;
};

const mapping = new Map([...classNames]
	.sort((left, right) => left.localeCompare(right))
	.map((name, index) => [name, shortName(index)]));

const idMapping = new Map([...idNames]
	.sort((left, right) => left.localeCompare(right))
	.map((name, index) => [name, shortName(index)]));

const remapClassList = (value) => value
	.split(/\s+/)
	.map((name) => mapping.get(name) || name)
	.join(' ');

const remapIds = (value) => idMapping.get(value) || value;

const remappedHtml = html
	.replace(/\bclass="([^"]+)"/g, (_, value) => `class="${remapClassList(value)}"`)
	.replace(/\bid="([^"]+)"/g, (_, value) => `id="${remapIds(value)}"`)
	.replace('<link rel="stylesheet" href="styles.css" />', '<link rel="stylesheet" href="styles.min.css" />')
	.replace('<script defer src="script.js"></script>', '<script defer src="script.min.js"></script>');

const remappedCss = css.replace(/\.([A-Za-z_][A-Za-z0-9_-]*)/g, (full, name) => mapping.has(name) ? `.${mapping.get(name)}` : full);
let remappedJs = [...idMapping.entries()].reduce(
	(output, [original, short]) => output.replaceAll(`"${original}"`, `"${short}"`).replaceAll(`'${original}'`, `'${short}'`),
	js
).replace(/className\s*=\s*(['"])([^'"]+)\1/g, (_, quote, value) => `className = ${quote}${remapClassList(value)}${quote}`);

fs.writeFileSync('/dist/index.raw.html', remappedHtml);
fs.writeFileSync('/dist/styles.raw.css', remappedCss);
fs.writeFileSync('/dist/script.raw.js', remappedJs);
NODE

cleancss -o /dist/styles.min.css /dist/styles.raw.css
terser /dist/script.raw.js -c passes=2,toplevel=true -m toplevel=true -o /dist/script.min.js

node <<'NODE'
const fs = require('fs');

const html = fs.readFileSync('/dist/index.raw.html', 'utf8')
	.replace('<link rel="stylesheet" href="styles.min.css" />', `<style>${fs.readFileSync('/dist/styles.min.css', 'utf8')}</style>`)
	.replace('<script defer src="script.min.js"></script>', `<script>${fs.readFileSync('/dist/script.min.js', 'utf8')}</script>`);

fs.writeFileSync('/dist/index.inlined.html', html);
NODE

html-minifier-terser \
	--collapse-whitespace \
	--remove-comments \
	--remove-redundant-attributes \
	--remove-optional-tags \
	--remove-tag-whitespace \
	--remove-script-type-attributes \
	--remove-style-link-type-attributes \
	--sort-attributes \
	--sort-class-name \
	--use-short-doctype \
	--minify-css true \
	--minify-js true \
	-o /dist/index.html \
	/dist/index.inlined.html
EOF

# -------- Go Build Stage --------
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .
COPY --from=frontend-builder /dist/index.html ./frontend/dist/index.html

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

# -------- Final Stage --------
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]

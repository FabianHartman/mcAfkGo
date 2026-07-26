const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const frontendDir = __dirname;
const distDir = path.join(frontendDir, 'dist');

if (!fs.existsSync(distDir)) {
  fs.mkdirSync(distDir, { recursive: true });
}

const html = fs.readFileSync(path.join(frontendDir, 'index.html'), 'utf8');
const css = fs.readFileSync(path.join(frontendDir, 'styles.css'), 'utf8');
const js = fs.readFileSync(path.join(frontendDir, 'script.js'), 'utf8');

const classNames = new Set();
const idNames = new Set();

for (const match of html.matchAll(/\bclass="([^"]+)"/g)) {
  for (const name of match[1].trim().split(/\s+/)) {
    if (name) classNames.add(name);
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
    if (name) classNames.add(name);
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

fs.writeFileSync(path.join(distDir, 'index.raw.html'), remappedHtml);
fs.writeFileSync(path.join(distDir, 'styles.raw.css'), remappedCss);
fs.writeFileSync(path.join(distDir, 'script.raw.js'), remappedJs);

try {
  execSync(`npx clean-css-cli -o ${path.join(distDir, 'styles.min.css')} ${path.join(distDir, 'styles.raw.css')}`, { stdio: 'inherit' });
  execSync(`npx terser ${path.join(distDir, 'script.raw.js')} -c passes=2,toplevel=true -m toplevel=true -o ${path.join(distDir, 'script.min.js')}`, { stdio: 'inherit' });

  const inlinedHtml = fs.readFileSync(path.join(distDir, 'index.raw.html'), 'utf8')
    .replace('<link rel="stylesheet" href="styles.min.css" />', `<style>${fs.readFileSync(path.join(distDir, 'styles.min.css'), 'utf8')}</style>`)
    .replace('<script defer src="script.min.js"></script>', `<script>${fs.readFileSync(path.join(distDir, 'script.min.js'), 'utf8')}</script>`);

  fs.writeFileSync(path.join(distDir, 'index.inlined.html'), inlinedHtml);

  execSync(`npx html-minifier-terser --collapse-whitespace --remove-comments --remove-redundant-attributes --remove-optional-tags --remove-tag-whitespace --remove-script-type-attributes --remove-style-link-type-attributes --sort-attributes --sort-class-name --use-short-doctype --minify-css true --minify-js true -o ${path.join(distDir, 'index.html')} ${path.join(distDir, 'index.inlined.html')}`, { stdio: 'inherit' });

  console.log('Frontend minification completed successfully.');
} catch (err) {
  console.error('Build failed:', err);
  process.exit(1);
}

# bundle js

cat theme/default/theme.html > ./dist/theme.html

wget -O alpinejs@persist.js https://cdn.jsdelivr.net/npm/@alpinejs/persist@3.x.x/dist/cdn.min.js
wget -O alpinejs.js https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js

echo "import './alpinejs@persist.js'\nimport './alpinejs.js'\n" > .import.js

cat  theme/default/theme.js >> .import.js

./esbuild .import.js --bundle --minify-whitespace --outfile=dist/theme.js

rm -f .import.js
rm -f alpinejs@persist.js
rm -f alpinejs.js


# bundle css

wget -O pico.css https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css

echo '@import "pico.css";\n' > .import.css

cat  theme/default/theme.css >> .import.css

./esbuild .import.css --bundle --minify-whitespace --outfile=dist/theme.css

rm -f .import.css
rm -f pico.css
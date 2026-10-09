# README roundtable artwork

Generated with the built-in `image_gen` tool. The README embeds the ASCII version; the PNG is the retained source illustration.

## Generation prompt

```text
Use case: illustration-story
Asset type: source illustration to be converted into ASCII art for agent-relay's GitHub README.
Primary request: three distinct friendly coding-agent robot mascots representing Codex, Claude, and OpenCode sitting together at a round table, collaborating on code.
Scene: plain pure white background, no room, no decorative scenery.
Subjects: exactly three small expressive robots, left a box-headed Codex robot with a simple terminal-face >_, center a Claude robot with a round starburst head, right an OpenCode robot with an angular helmet-like head and two square eyes. All seated visibly around the SAME broad oval meeting table, viewed from the front at a slight elevation. Their upper bodies and hands are clearly visible above or alongside the table. One simple small laptop per robot, minimal terminal symbols.
Style: extremely simple black ink silhouette and thick outline illustration, pure black and white, large clean shapes, no gray shading, no hatching, no texture. The robots should remain recognisable when reduced to about 90 columns by 32 text rows. Use thick outlines and generous spacing between subjects. Wide landscape composition, close crop with a small white margin.
Text: no words, no labels, no fine text; terminal face glyphs only.
Constraints: strong readable silhouettes; clearly oval tabletop plus short legs below; no watermark, no extra characters, no fine detail.
```

## Conversion

Converted with Pillow to grayscale, then downsampled with Lanczos to 100 columns and a row count equal to `round(height / width * 100 * 0.57)` to compensate for monospace character proportions. Each pixel maps to ` .:+#@` using `max(0, min(5, round((245 - gray) / 245 * 5)))`. Blank outer rows and trailing spaces are removed. Harness labels are added as plain text above the converted image.

The `roundtable.txt` file exactly matches the README fenced block. The source is a conceptual mascot illustration, rather than official harness logos.


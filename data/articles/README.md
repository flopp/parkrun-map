# Article Data Structure

Each article lives in its own directory under `data/articles`:

- `data/articles/<slug>/article.json`: metadata
- `data/articles/<slug>/content.html`: article body (trusted HTML)
- `data/articles/<slug>/...`: local assets used by the article (images, downloads)

Example:

```
data/articles/
  neuen-parkrun-starten/
    article.json
    content.html
    cover.svg
```

## `article.json` fields

- `slug` (string, optional): URL slug, defaults to folder name
- `title` (string, required): article headline
- `summary` (string, required): short preview text for list pages and meta description
- `image` (string, optional): filename of a local image inside the article folder
- `published` (string, optional): publication date in `YYYY-MM-DD`
- `updated` (string, optional): last update date in `YYYY-MM-DD`
- `tags` (string array, optional): visible topic labels

Article content is always loaded from `content.html` in the same folder.

## Published URLs

- Article list: `/articles/`
- Article detail: `/articles/<slug>.html`
- Article assets: `/articles/<slug>/<filename>`

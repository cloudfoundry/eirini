# urljoiner

A utility for joining together path segments into URLs.

Import via `code.cloudfoundry.org/urljoiner`.


## Example

```
url := urljoiner.Join("https://example.com", "foo", "bar")
# produces "https://example.com/foo/bar"
```

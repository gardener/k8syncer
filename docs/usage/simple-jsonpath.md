# Simple JSONPath

Some fields in the K8Syncer configuration expect a reference to a specific field in a k8s manifest as a value. The syntax to specify these references is a primitive version of the JSONPath syntax:

- Field names are separated by `.`. There is no preceding `.` before the first field name.
- Only fields of objects (maps) can be referenced. Referencing an entry of a list is not possible.
- To reference a field that contains a `.` in its name, escaping a `.` is possible via `\.`.
- To reference a field that ends with `\`, use double escapes `\\`.
- Escapes `\` are only evaluated if they directly precede a `.`. A `\` in the middle of a field name will be taken as is.

## Examples

#### Example 1
`a.b.c`
```yaml
a:
  b:
    c <=
```

#### Example 2

Use `\` to escape a `.` in the field name.

`a\.b.c`
```yaml
a.b:
  c <=
```

#### Example 3

Use `\\` to escape a `\` preceding a `.`.

`a\\.b.c`
```yaml
a\:
  b:
    c <=
```

#### Example 4

Escapes are only evaluated if they directly precede a `.`.

`a\a.b.c\`
```yaml
a\a:
  b:
    c\ <=
```

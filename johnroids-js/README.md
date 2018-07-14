This is a driver for johnroids for gopher js and go/wasm.  There is a
very thin abstraction layer for the two different systems.

## Performance

Times to plot the opening screen


|            | gopherjs | wasm    |
|:-----------| --------:|--------:|
| Chrome 67  |  7.2 mS  | 10.1 mS |
| Firefox 60 | 10.9 mS  |  5.0 mS |

# badger
---

badger is a tool that helps you filter large folders of photos. It:

- Copies media into a directory, with photos / video clustered together by the time they were taken
- Updates photo names with an estimate of how blurry they are, so you can sort from likey sharpest to blurriest within a burst of photos.
- Flattens the final selection of media (after you delete what you don't want to keep) into a folder ready to upload.

It has a few niceties:

- Detailed progress statistics
- Concurrent & database-based to minimise processing time
- Groups raw images alongside matching jpeg images when present

## Usage

```bash
badger cluster --from '/media/rg/3236-3061/DCIM/**/*' --to '/home/rg/Desktop/resources' --max-seconds-diff 4
```

## License

The MIT License

Copyright (c) 2021 Róisín Grannell

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

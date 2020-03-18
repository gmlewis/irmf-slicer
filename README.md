# [IRMF Shader](https://github.com/gmlewis/irmf) Slicer

[![Test Status](https://github.com/gmlewis/irmf-slicer/workflows/Go/badge.svg)](https://github.com/gmlewis/irmf-slicer/actions?query=workflow%3AGo)

![IRMF slicer demo](irmf-slicer-demo.gif)

## Summary

IRMF is a file format used to describe [GLSL
ES](https://en.wikipedia.org/wiki/OpenGL_ES) shaders that define the
materials in a 3D object with infinite resolution. IRMF completely
eliminates the need for [software
slicers](https://en.wikipedia.org/wiki/Slicer_(3D_printing)),
[STL](https://en.wikipedia.org/wiki/STL_(file_format)), and
[G-code](https://en.wikipedia.org/wiki/G-code) files used in
[3D printers](https://en.wikipedia.org/wiki/3D_printing).

I believe that IRMF shaders will revolutionize the 3D-printing industry.

See [github.com/gmlewis/irmf](https://github.com/gmlewis/irmf) for more
details.

## About the IRMF Shader Slicer

The technology stack used is Go and OpenGL.

This program is needed to bridge the gap until 3D printer manufacturers
adopt IRMF shaders and printable file formats in addition to G-Code
or voxel slices. It slices an IRMF shader model into either STL files
or into voxel slices. For STL files, it outputs one STL file per material.
(Note that some STL files can become enormous, way larger than any online
service bureau currently supports. The resolution can be reduced to limit
the STL file sizes, but at the expense of lossed detail.)

For voxel slices, it writes them out to one ZIP file per material.
These slices can then be fed to 3D printer software that accepts
voxel slices as input for printing (such as [NanoDLP](https://www.nanodlp.com/)).

Once 3D printers support IRMF shader model files directly for printing,
this slicer will no longer be needed.

# FAQ

## How do I install it?

After you have a recent version of [Go](https://golang.org/) installed,
run the following command in a terminal window:

```sh
$ go install github.com/gmlewis/irmf-slicer/cmd/irmf-slicer
```

(Prior to Go 1.13, you would use:

```sh
$ GO111MODULE=on go install github.com/gmlewis/irmf-slicer/cmd/irmf-slicer
```
)

Then you might want to try it out on some of the [example IRMF
shaders](https://github.com/gmlewis/irmf#examples) located on GitHub.

To slice one or more `.irmf` files, just list them on the command line,
like this:

```sh
$ irmf-slicer -view -stl examples/*/*.irmf
```

## How does it work?

This slicer dices up your model (the IRMF shader) into slices (planes)
that are perpendicular (normal) to the Z (up) axis. The slices are very
thin and when stacked together, represent your solid model.

Using the `-zip` option, the result is a ZIP file per model material
with all the slices in the root of the ZIP so as to be compatible
with NanoDLP. When using the `-zip` option, the resolution is set
to X: 65, Y: 60, Z: 30 microns (unless the `-res` option is used to
override this) in order to support the `MCAST + Sylgard / 65 micron`
option of NanoDLP.

Using the `-stl` option, the result is one STL file per model material.

## Why do I get a `Slice: compile shader` error?

Hmmm... Does the Mac not support GLSL ES 3.00?
This needs more investigation.

----------------------------------------------------------------------

# License

Copyright 2019 Glenn M. Lewis. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

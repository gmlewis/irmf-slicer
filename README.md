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

I believe that IRMF shaders will some day revolutionize the 3D-printing industry.

See [github.com/gmlewis/irmf](https://github.com/gmlewis/irmf) for more
details.

## LYGIA support

As of 2022-10-27, support has been added for using the LYGIA Shader Library
at: https://lygia.xyz !

This means that you can add lines to your IRMF shaders like this:

```glsl
#include "lygia/math/decimation.glsl"
```

and the source will be retrieved from the LYGIA server.

Congratulations and thanks go to [Patricio Gonzalez Vivo](https://github.com/sponsors/patriciogonzalezvivo)
for making the LYGIA server available for anyone to use, and also
for the amazing tool [glslViewer](https://github.com/patriciogonzalezvivo/glslViewer)!

## About the IRMF Shader Slicer

The technology stack used is Go and OpenGL.

This program is needed to bridge the gap until 3D printer manufacturers
adopt IRMF shaders and printable file formats in addition to G-Code
or voxel slices. It slices an IRMF shader model into either STL files
or into voxel slices (with various output file formats).
For STL files, it outputs one STL file per material.
(Note that some STL files can become enormous, way larger than any online
service bureau currently supports. The resolution can be reduced to limit
the STL file sizes, but at the expense of lossed detail.)

For voxel slices, it can write them out to ZIP files (one ZIP file per material).
These slices can then be fed to 3D printer software that accepts
voxel slices as input for printing (such as [NanoDLP](https://www.nanodlp.com/)).

For resin printers using either the [ChiTuBox](https://www.chitubox.com/) or
[AnyCubic](https://www.anycubic.com/products/anycubic-photon-3d-printer) slicer
(such as the [Elegoo Mars](https://www.elegoo.com/product/elegoo-mars-uv-photocuring-lcd-3d-printer/)),
the `-dlp` option will output the voxel slices to the `.cbddlp` file
format (which is identical to the `.photon` file format).

Once 3D printers support IRMF shader model files directly for printing,
however, this slicer will no longer be needed.

# FAQ

## How do I install it?

After you have a recent version of [Go](https://golang.org/) installed,
run the following command in a terminal window:

```sh
$ go install github.com/gmlewis/irmf-slicer/cmd/irmf-slicer
```

Then you might want to try it out on some of the [example IRMF
shaders](https://github.com/gmlewis/irmf#examples) located on GitHub.

To slice one or more `.irmf` files, just list them on the command line,
like this:

```sh
$ irmf-slicer -view -stl examples/*/*.irmf
```

The output files will be saved in the same directory as the original
input IRMF files.

## How does it work?

This slicer dices up your model (the IRMF shader) into slices (planes)
that are perpendicular (normal) to the Z (up) axis. The slices are very
thin and when stacked together, represent your solid model.

Using the `-zip` option, the result is one ZIP file per model material
with all the slices in the root of the ZIP so as to be compatible
with NanoDLP. When using the `-zip` option, the resolution is set
to X: 65, Y: 60, Z: 30 microns (unless the `-res` option is used to
override this) in order to support the `MCAST + Sylgard / 65 micron`
option of NanoDLP.

Using the `-dlp` option, the result is one `.cbddlp` file per model material
that can be loaded into the [ChiTuBox](https://www.chitubox.com/) or
[AnyCubic](https://www.anycubic.com/products/anycubic-photon-3d-printer)
slicer directly (`.cbddlp` is identical to the `.photon` file format).

Using the `-stl` option, the result is one STL file per model material.

Using the `-binvox` option, it will write one `.binvox` file per model material.

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

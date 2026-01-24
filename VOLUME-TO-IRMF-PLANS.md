# Volume-to-IRMF Algorithm Implementation Plan

## Goal
To implement a new package `volume-to-irmf` that converts 3D voxelized models into optimized IRMF shaders (WGSL) using a Reinforcement Learning (RL) approach assisted by WebGPU compute shaders.

## Package: `volume-to-irmf`

### 1. Data Structures
- `Primitive`: Represents a sphere or axis-aligned cube.
  - Type (Sphere/Cube)
  - Center (x, y, z)
  - Size (radius for sphere, half-extents for cube)
  - Operation (Union/Difference)
- `Model`: A collection of `Primitive` objects.
- `VoxelGrid`: Internal representation of the target 3D model (likely `*binvox.BinVOX`).

### 2. Reinforcement Learning (RL) Algorithm
- **Agent**: Modifies the `Model` by adding, removing, or tweaking `Primitive` parameters.
- **State**: The current list of primitives and their parameters.
- **Action Space**:
  - Add a sphere/cube (Union/Difference).
  - Remove a primitive.
  - Move/Resize a primitive.
- **Reward Function**: Based on the hybrid "MSE + IoU" error function defined in `ReinforcementLearningErrorFunc.md`.
  - $R = -(\alpha \cdot \text{MSE} + \beta \cdot (1 - \text{IoU}))$
- **Optimization Strategy**:
  - Start with a "divide-and-conquer" approach to find the best initial primitives.
  - Iteratively refine using stochastic sampling or gradient-free optimization (e.g., hill climbing or genetic algorithms).

### 3. WebGPU Compute Shader
- **Purpose**: Fast parallel evaluation of the error function.
- **Inputs**:
  - Target voxel grid (Bitset or 3D Texture).
  - Current primitives (Uniform buffer or Storage buffer).
- **Process**:
  - Each invocation represents a voxel or a group of voxels.
  - Calculate the IRMF value (0.0 to 1.0) at the voxel center.
  - Compare with the target voxel value.
  - Accumulate MSE and IoU components (Intersection count, Union count, Squared Error).
- **Output**: Summed error components for the entire grid (using reduction).

### 4. IRMF Generation
- Convert the optimized `Model` (list of primitives) into a valid WGSL `mainModel4` (or `mainModel9`/`mainModel16`) function.
- Primitives will be represented as simple math functions (e.g., `length(p - center) - radius` for spheres).
- Boolean operations will use `max`/`min` or basic math as described in `IRMF.md`.

## Executables

Each executable will follow this pattern:
1. Load the specific format.
2. Voxelize (if not already voxels).
3. Run `volume-to-irmf` optimization.
4. Save the resulting `.irmf` file.

### 1. `binvox-to-irmf`
- Reads `.binvox` files.
- Directly uses the voxels for optimization.

### 2. `stl-to-irmf`
- Reads `.stl` files.
- Voxelizes the STL mesh into a `BinVOX` structure at the requested resolution.

### 3. `svx-to-irmf` / `zip-to-irmf`
- Reads `.svx` or `.zip` files containing slices.
- Reconstructs the 3D voxel grid from image slices.

### 4. `dlp-to-irmf`
- Reads DLP-specific slice formats.
- Reconstructs voxels for optimization.

## Implementation Steps

1. **Scaffold `volume-to-irmf`**: Define core types and the RL agent interface.
2. **Develop WebGPU Compute Shader**: Implement the error function calculation.
3. **Implement RL Optimizer**: Start with the "divide-and-conquer" strategy.
4. **IRMF Generator**: Write logic to produce WGSL from the `Model`.
5. **Implement Executables**: Start with `binvox-to-irmf`.
6. **Tests**: Add unit tests for each component, especially the error function and primitive math.

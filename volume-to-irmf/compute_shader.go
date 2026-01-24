package volumetoirmf

const computeShaderSource = `
struct Primitive {
    p_type: u32,
    centerX: f32,
    centerY: f32,
    centerZ: f32,
    size: f32,
    operation: u32,
};

struct ModelData {
    num_primitives: u32,
    paddingX: u32,
    paddingY: u32,
    paddingZ: u32,
    primitives: array<Primitive, 1024>,
};

struct Stats {
    mse: f32,
    intersection: f32,
    union_val: f32,
};

@group(0) @binding(0) var<storage, read> target_voxels: array<u32>;
@group(0) @binding(1) var<storage, read> model: ModelData;
@group(0) @binding(2) var<storage, read_write> results: array<Stats>;

struct Config {
    nx: u32,
    ny: u32,
    nz: u32,
    voxel_size: f32,
    minX: f32,
    minY: f32,
    minZ: f32,
    padding: f32,
};

@group(0) @binding(3) var<uniform> config: Config;

var<workgroup> wg_mse: array<f32, 256>;
var<workgroup> wg_inter: array<f32, 256>;
var<workgroup> wg_union: array<f32, 256>;

fn sdSphere(p: vec3f, r: f32) -> f32 {
    return length(p) - r;
}

fn sdBox(p: vec3f, b: vec3f) -> f32 {
    let q = abs(p) - b;
    return length(max(q, vec3f(0.0))) + min(max(q.x, max(q.y, q.z)), 0.0);
}

fn get_target(x: u32, y: u32, z: u32) -> f32 {
    if (x >= config.nx || y >= config.ny || z >= config.nz) { return 0.0; }
    let idx = z * (config.nx * config.ny) + y * config.nx + x;
    let byte_idx = idx / 32;
    let bit_idx = idx % 32;
    if ((target_voxels[byte_idx] & (1u << bit_idx)) != 0u) {
        return 1.0;
    }
    return 0.0;
}

@compute @workgroup_size(8, 8, 4)
fn main(
    @builtin(global_invocation_id) global_id: vec3u,
    @builtin(local_invocation_index) local_idx: u32,
    @builtin(workgroup_id) wg_id: vec3u,
    @builtin(num_workgroups) num_wgs: vec3u
) {
    let x = global_id.x;
    let y = global_id.y;
    let z = global_id.z;

    var mse = 0.0;
    var inter = 0.0;
    var uni = 0.0;

    if (x < config.nx && y < config.ny && z < config.nz) {
        let p = vec3f(config.minX, config.minY, config.minZ) + vec3f(global_id) * config.voxel_size + vec3f(config.voxel_size * 0.5);
        
        var d = 1e10;
        for (var i = 0u; i < model.num_primitives; i = i + 1u) {
            let prim = model.primitives[i];
            let center = vec3f(prim.centerX, prim.centerY, prim.centerZ);
            var dist = 0.0;
            if (prim.p_type == 0u) { // Sphere
                dist = sdSphere(p - center, prim.size);
            } else { // Cube
                dist = sdBox(p - center, vec3f(prim.size));
            }

            if (i == 0u) {
                d = dist;
            } else {
                if (prim.operation == 0u) { // Union
                    d = min(d, dist);
                } else { // Difference
                    d = max(d, -dist);
                }
            }
        }

        let trgt = get_target(x, y, z);
        let current = select(0.0, 1.0, d <= 0.0);

        mse = (trgt - current) * (trgt - current);
        inter = min(trgt, current);
        uni = max(trgt, current);
    }

    wg_mse[local_idx] = mse;
    wg_inter[local_idx] = inter;
    wg_union[local_idx] = uni;

    workgroupBarrier();

    // Reduction within workgroup
    for (var i = 128u; i > 0u; i = i >> 1u) {
        if (local_idx < i) {
            wg_mse[local_idx] = wg_mse[local_idx] + wg_mse[local_idx + i];
            wg_inter[local_idx] = wg_inter[local_idx] + wg_inter[local_idx + i];
            wg_union[local_idx] = wg_union[local_idx] + wg_union[local_idx + i];
        }
        workgroupBarrier();
    }

    if (local_idx == 0u) {
        let wg_idx = wg_id.z * (num_wgs.x * num_wgs.y) + wg_id.y * num_wgs.x + wg_id.x;
        results[wg_idx].mse = wg_mse[0];
        results[wg_idx].intersection = wg_inter[0];
        results[wg_idx].union_val = wg_union[0];
    }
}
`

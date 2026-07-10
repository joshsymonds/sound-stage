"""Pure song-list sharding for the RunPod backfill supervisor.

Kept free of subprocess/SSH/API calls so shard assignment is unit-testable
without touching RunPod or the filesystem.
"""


def shard_songs(song_names: list[str], n_shards: int) -> list[list[str]]:
    """Split song_names into n_shards round-robin, preserving encounter order
    within each shard. Round-robin (rather than contiguous slicing) keeps
    shard sizes balanced within one song even when n_shards doesn't evenly
    divide the song count.
    """
    if n_shards < 1:
        raise ValueError(f"n_shards must be >= 1, got {n_shards}")

    shards: list[list[str]] = [[] for _ in range(n_shards)]
    for i, name in enumerate(song_names):
        shards[i % n_shards].append(name)
    return shards

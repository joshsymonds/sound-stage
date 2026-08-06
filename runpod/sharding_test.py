"""Tests for runpod/sharding.py — pure song-list sharding."""

import pytest

from runpod.sharding import shard_songs


class TestShardSongs:
    def test_even_split_preserves_all_songs(self) -> None:
        songs = [f"song-{i}" for i in range(9)]
        shards = shard_songs(songs, 3)
        assert len(shards) == 3
        assert sorted(name for shard in shards for name in shard) == sorted(songs)

    def test_balanced_within_one(self) -> None:
        songs = [f"song-{i}" for i in range(11)]
        shards = shard_songs(songs, 3)
        sizes = [len(s) for s in shards]
        assert max(sizes) - min(sizes) <= 1

    def test_deterministic_ordering(self) -> None:
        songs = [f"song-{i}" for i in range(20)]
        first = shard_songs(songs, 4)
        second = shard_songs(songs, 4)
        assert first == second

    def test_more_shards_than_songs_yields_empty_shards(self) -> None:
        songs = ["a", "b"]
        shards = shard_songs(songs, 5)
        assert len(shards) == 5
        assert sorted(name for shard in shards for name in shard) == ["a", "b"]
        assert sum(1 for s in shards if s == []) == 3

    def test_empty_song_list(self) -> None:
        shards = shard_songs([], 4)
        assert shards == [[], [], [], []]

    def test_single_shard_contains_everything_in_order(self) -> None:
        songs = ["c", "a", "b"]
        shards = shard_songs(songs, 1)
        assert shards == [["c", "a", "b"]]

    def test_rejects_zero_shards(self) -> None:
        with pytest.raises(ValueError):
            shard_songs(["a"], 0)

    def test_rejects_negative_shards(self) -> None:
        with pytest.raises(ValueError):
            shard_songs(["a"], -1)

    def test_no_duplicate_assignment(self) -> None:
        songs = [f"song-{i}" for i in range(37)]
        shards = shard_songs(songs, 7)
        flat = [name for shard in shards for name in shard]
        assert len(flat) == len(set(flat)) == len(songs)

"""Tests for delyric.py — update_song_txt and is_processed logic."""

import pytest
from pathlib import Path

from delyric import is_processed, update_song_txt


@pytest.fixture
def song_dir(tmp_path: Path) -> Path:
    """Create a minimal song directory with audio.webm and song.txt."""
    d = tmp_path / "Artist - Title"
    d.mkdir()
    (d / "audio.webm").write_bytes(b"fake audio")
    (d / "song.txt").write_text(
        "#ARTIST:Test Artist\n"
        "#TITLE:Test Title\n"
        "#MP3:audio.webm\n"
        "#VIDEO:video.webm\n"
        "#COVER:cover.jpg\n"
        "#BPM:120\n"
        "#GAP:1000\n"
        ": 0 2 13 Test\n",
        encoding="utf-8",
    )
    return d


class TestIsProcessed:
    def test_unprocessed(self, song_dir: Path) -> None:
        assert not is_processed(song_dir)

    def test_only_instrumental(self, song_dir: Path) -> None:
        (song_dir / "instrumental.webm").write_bytes(b"fake")
        assert not is_processed(song_dir)

    def test_only_vocals(self, song_dir: Path) -> None:
        (song_dir / "vocals.webm").write_bytes(b"fake")
        assert not is_processed(song_dir)

    def test_both_present(self, song_dir: Path) -> None:
        (song_dir / "instrumental.webm").write_bytes(b"fake")
        (song_dir / "vocals.webm").write_bytes(b"fake")
        assert is_processed(song_dir)


class TestUpdateSongTxt:
    def test_inserts_tags_after_last_header(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        lines = content.split("\n")

        # Tags should be after #GAP (last header) and before lyrics
        gap_idx = next(i for i, l in enumerate(lines) if l.startswith("#GAP:"))
        instrumental_idx = next(i for i, l in enumerate(lines) if l.startswith("#INSTRUMENTAL:"))
        vocals_idx = next(i for i, l in enumerate(lines) if l.startswith("#VOCALS:"))
        lyrics_idx = next(i for i, l in enumerate(lines) if l.startswith(": "))

        assert instrumental_idx == gap_idx + 1
        assert vocals_idx == gap_idx + 2
        assert lyrics_idx == gap_idx + 3

    def test_correct_tag_values(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:instrumental.webm" in content
        assert "#VOCALS:vocals.webm" in content

    def test_preserves_existing_content(self, song_dir: Path) -> None:
        original = (song_dir / "song.txt").read_text(encoding="utf-8")
        update_song_txt(song_dir)
        updated = (song_dir / "song.txt").read_text(encoding="utf-8")

        # All original lines should still be present
        for line in original.split("\n"):
            assert line in updated

    def test_idempotent(self, song_dir: Path) -> None:
        update_song_txt(song_dir)
        first = (song_dir / "song.txt").read_text(encoding="utf-8")
        update_song_txt(song_dir)
        second = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert first == second

    def test_skips_if_tags_already_present(self, song_dir: Path) -> None:
        # Manually add tags
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        content = content.replace(
            "#BPM:120\n",
            "#BPM:120\n#INSTRUMENTAL:instrumental.webm\n#VOCALS:vocals.webm\n",
        )
        (song_dir / "song.txt").write_text(content, encoding="utf-8")

        update_song_txt(song_dir)
        result = (song_dir / "song.txt").read_text(encoding="utf-8")
        # Should not duplicate tags
        assert result.count("#INSTRUMENTAL:") == 1
        assert result.count("#VOCALS:") == 1

    def test_adds_missing_vocal_tag_only(self, song_dir: Path) -> None:
        content = (song_dir / "song.txt").read_text(encoding="utf-8")
        content = content.replace("#BPM:120\n", "#BPM:120\n#INSTRUMENTAL:instrumental.webm\n")
        (song_dir / "song.txt").write_text(content, encoding="utf-8")

        update_song_txt(song_dir)
        result = (song_dir / "song.txt").read_text(encoding="utf-8")
        assert result.count("#INSTRUMENTAL:") == 1
        assert result.count("#VOCALS:") == 1

    def test_no_song_txt(self, tmp_path: Path) -> None:
        """Should not crash when song.txt is missing."""
        d = tmp_path / "No Txt Song"
        d.mkdir()
        update_song_txt(d)  # should log warning, not crash

    def test_headers_only_file(self, tmp_path: Path) -> None:
        """Handle song.txt that has only headers and no lyrics."""
        d = tmp_path / "Headers Only"
        d.mkdir()
        (d / "song.txt").write_text(
            "#ARTIST:Test\n#TITLE:Test\n#MP3:audio.webm\n",
            encoding="utf-8",
        )
        update_song_txt(d)
        content = (d / "song.txt").read_text(encoding="utf-8")
        assert "#INSTRUMENTAL:instrumental.webm" in content
        assert "#VOCALS:vocals.webm" in content

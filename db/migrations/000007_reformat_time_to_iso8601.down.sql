UPDATE infos
SET created_at = fixed.noniso
FROM (
	SELECT
        *,
        substr(created_at, 0, split_pos + 1) ||
        substr(created_at, split_pos + 1, 2) ||
        substr(created_at, split_pos + 4, 2) as noniso
    FROM (
        SELECT *, instr(substr(created_at, 12), " ") + 12 as split_pos
        FROM infos
    )
) AS fixed
WHERE infos.path = fixed.path;

UPDATE dirs
SET indexed_at = fixed.noniso
FROM (
	SELECT
        *,
        substr(indexed_at, 0, split_pos + 1) ||
        substr(indexed_at, split_pos + 1, 2) ||
        substr(indexed_at, split_pos + 4, 2) as noniso
    FROM (
        SELECT *, instr(substr(indexed_at, 12), " ") + 12 as split_pos
        FROM dirs
    )
) AS fixed
WHERE dirs.path = fixed.path;

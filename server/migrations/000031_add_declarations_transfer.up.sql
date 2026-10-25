-- "Declarations transfer" as a per-room rule, recorded on the match as well so
-- history can say which rule produced a score. Additive ALTER style, mirroring
-- 000022 (rooms) and 000023 (matches).
--
-- When the taker's team goes down, or a team is swept by a Capot, the side that
-- takes the whole hand has always banked every point on the table, including
-- the declarations and Belote the other side announced.
--
--     TRUE  -> those declarations go to the winners (today's behaviour).
--     FALSE -> the winners bank the card points plus only their own
--              declarations; the other side's declarations and Belote are voided.
--
-- NO BACKFILL IS NEEDED. DEFAULT TRUE reproduces every existing room and match
-- exactly. The Go structs declare the field WITHOUT a GORM `default` tag, the
-- same trap 000021 and 000023 document: GORM would omit a FALSE from the INSERT
-- and the column default would silently turn it back on.
ALTER TABLE rooms ADD COLUMN declarations_transfer BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE matches ADD COLUMN declarations_transfer BOOLEAN NOT NULL DEFAULT TRUE;

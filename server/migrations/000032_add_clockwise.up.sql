-- "Direction of play" as a per-room rule, recorded on the match as well so history
-- can say which direction a finished match was played in. Additive ALTER style,
-- mirroring 000031.
--
-- Seats 0, 1, 2, 3 sit counter-clockwise around the table, and play has always
-- moved to the next seat on the right: the deal, the bidding, the trick play and
-- the passing of the deal.
--
--     FALSE -> play moves counter-clockwise, to the right (today's behaviour).
--     TRUE  -> play moves clockwise, to the left. Seats stay where they are and
--              only who acts next changes.
--
-- NO BACKFILL IS NEEDED. DEFAULT FALSE reproduces every existing room and match
-- exactly. The Go structs declare the field WITHOUT a GORM `default` tag, the
-- same rule 000021 and 000023 document, so an explicit TRUE is always written.
ALTER TABLE rooms ADD COLUMN clockwise BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN clockwise BOOLEAN NOT NULL DEFAULT FALSE;

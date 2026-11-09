-- "Declare until trick" as a per-room rule, recorded on the match as well so history
-- can say how long a finished match's declaration window was. Additive ALTER style,
-- mirroring 000032.
--
-- In Bitola with declarations on, a player holding a meld may declare before their
-- card in any of tricks 1..N. The value is N, from 1 to 3. Every other room stores 3.
--
-- NO BACKFILL IS NEEDED. DEFAULT 3 is the standard window, so every existing room
-- and match reads as 3.
ALTER TABLE rooms ADD COLUMN declaration_last_trick SMALLINT NOT NULL DEFAULT 3;
ALTER TABLE matches ADD COLUMN declaration_last_trick SMALLINT NOT NULL DEFAULT 3;

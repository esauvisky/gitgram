-- reaction is the emoji the bot currently keeps on the card message, so
-- the sender only calls setMessageReaction when the status changes it.
ALTER TABLE card_messages ADD COLUMN reaction TEXT NOT NULL DEFAULT '';

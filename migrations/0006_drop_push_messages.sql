-- Push cards are object_state rows now, and the run-pipeline button carries
-- its branch in the callback, so the message → branch table is dead.
DROP TABLE IF EXISTS push_messages;

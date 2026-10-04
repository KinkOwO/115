ALTER TABLE characters ADD CONSTRAINT reject_graduation CHECK(NOT(state?'odyssey_graduation_version'));

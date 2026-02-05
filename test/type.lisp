(in-package :acc/test)

(fiveam:def-suite type)
(fiveam:in-suite type)

(fiveam:test test-type-error
  (fiveam:signals error (acc::parse-type (acc::make-token-sequence (acc::tokenize "100"))))
  (fiveam:signals error (acc::parse-type (acc::make-token-sequence (acc::tokenize "abc 123"))))
  (fiveam:signals error (acc::parse-type (acc::make-token-sequence (acc::tokenize "{hello}")))))

(fiveam:test test-atomic-type
  (fiveam:is (eq :int8 (acc::integer-type-size (acc::parse-type (acc::make-token-sequence (acc::tokenize "int8"))))))
  (fiveam:is (eq :int64 (acc::integer-type-size (acc::parse-type (acc::make-token-sequence (acc::tokenize "int64"))))))
  (fiveam:is (eq :int32 (acc::integer-type-size (acc::parse-type (acc::make-token-sequence (acc::tokenize "int"))))))
  (fiveam:is (eq :int16 (acc::integer-type-size (acc::parse-type (acc::make-token-sequence (acc::tokenize "int16"))))))
  (fiveam:is (eq :int32 (acc::integer-type-size (acc::parse-type (acc::make-token-sequence (acc::tokenize "int32")))))))
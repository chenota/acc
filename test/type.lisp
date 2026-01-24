(in-package :acc)

(fiveam:def-suite type)

(fiveam:test test-type-error
  (fiveam:signals error (parse-type (make-token-sequence (tokenize "100"))))
  (fiveam:signals error (parse-type (make-token-sequence (tokenize "abc 123"))))
  (fiveam:signals error (parse-type (make-token-sequence (tokenize "{hello}")))))

(fiveam:test test-atomic-type
  (fiveam:is (eq :char (integer-type-size (parse-type (make-token-sequence (tokenize "char"))))))
  (fiveam:is (eq :int64 (integer-type-size (parse-type (make-token-sequence (tokenize "int64"))))))
  (fiveam:is (eq :int32 (integer-type-size (parse-type (make-token-sequence (tokenize "int"))))))
  (fiveam:is (eq :int16 (integer-type-size (parse-type (make-token-sequence (tokenize "int16"))))))
  (fiveam:is (eq :int32 (integer-type-size (parse-type (make-token-sequence (tokenize "int32")))))))
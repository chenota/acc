(in-package :acc)

(fiveam:def-suite program)

(fiveam:test test-program
  (fiveam:is (program-node-p (parse-program (make-token-sequence (tokenize "func main int { return 0; }")))))
  (fiveam:signals error (parse-program (make-token-sequence (tokenize "return 0;")))))

(fiveam:test test-func
  (fiveam:is (function-node-p (function-rule (make-token-sequence (tokenize "func main int { return 0; }")) (list 0 0))))
  (fiveam:is (string= "main" (function-node-name (function-rule (make-token-sequence (tokenize "func main int { return 0; }")) (list 0 0)))))
  (fiveam:is (eq :int64 (primitive-type-kind (function-node-return-type (function-rule (make-token-sequence (tokenize "func main int { return 0; }")) (list 0 0))))))
  (fiveam:is (not (function-rule (make-token-sequence (tokenize "func { return 0; }")) (list 0 0)))))

(fiveam:test test-stmt
  (fiveam:is (return-statement-node-p (stmt-rule (make-token-sequence (tokenize "return 0;")) (list 0 0))))
  (fiveam:is (not (stmt-rule (make-token-sequence (tokenize "return return return")) (list 0 0)))))
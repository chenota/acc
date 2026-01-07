(in-package :acc)

(fiveam:def-suite program)

(fiveam:test test-program
  (fiveam:is (eq :program (car (parse-program (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (fiveam:signals error (parse-program (make-token-sequence (tokenize "return 0;")))))

(fiveam:test test-func
  (fiveam:is (eq :func (car (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (fiveam:is (string= "main" (cadr (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (fiveam:is (string= "int" (caddr (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (fiveam:is (not (function-rule (make-token-sequence (tokenize "func { return 0; }"))))))

(fiveam:test test-stmt
  (fiveam:is (eq :return (car (stmt-rule (make-token-sequence (tokenize "return 0;"))))))
  (fiveam:is (not (stmt-rule (make-token-sequence (tokenize "return return return"))))))
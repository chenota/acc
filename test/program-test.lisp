(in-package :acc)

(fiveam:def-suite program)

(test test-program
  (is (eq :program (car (parse-program (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (signals error (parse-program (make-token-sequence (tokenize "return 0;")))))

(test test-func
  (is (eq :func (car (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (is (string= "main" (cadr (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (is (string= "int" (caddr (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))))))
  (is (not (function-rule (make-token-sequence (tokenize "func { return 0; }"))))))

(test test-stmt
  (is (eq :return (car (stmt-rule (make-token-sequence (tokenize "return 0;"))))))
  (is (not (stmt-rule (make-token-sequence (tokenize "return return return"))))))
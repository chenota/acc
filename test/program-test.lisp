(in-package :acc)

(fiveam:def-suite program)

(test test-main-func
  (is (parse-program (make-token-sequence (tokenize "func main int { return 0; }")))))
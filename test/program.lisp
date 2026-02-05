(in-package :acc)

(fiveam:def-suite program)
(fiveam:in-suite program)

(fiveam:test test-program
  (fiveam:is (program-node-p (parse-program (make-token-sequence (tokenize "fun main int { return 0; }")))))
  (fiveam:signals error (parse-program (make-token-sequence (tokenize "return 0;")))))

(fiveam:test test-fun
  (fiveam:is (function-node-p (function-rule (make-token-sequence (tokenize "fun main int { return 0; }")))))
  (fiveam:is (string= "main" (function-node-name (function-rule (make-token-sequence (tokenize "fun main int { return 0; }"))))))
  (fiveam:is (eq :int32 (integer-type-size (function-node-return-type (function-rule (make-token-sequence (tokenize "fun main int { return 0; }")))))))
  (fiveam:is (not (function-rule (make-token-sequence (tokenize "fun { return 0; }"))))))

(fiveam:test test-stmt
  (fiveam:is (return-statement-node-p (stmt-rule (make-token-sequence (tokenize "return 0;")))))
  (fiveam:is (declaration-node-p (stmt-rule (make-token-sequence (tokenize "let x : int = 0;")))))
  (fiveam:is (assignment-node-p (stmt-rule (make-token-sequence (tokenize "x = 1;")))))
  (fiveam:is (not (stmt-rule (make-token-sequence (tokenize "return return return"))))))

(defun block-from (str)
  (fiveam:finishes (block-rule (make-token-sequence (tokenize str)))))

(fiveam:test block-empty
  (fiveam:is (block-node-p (block-from "{}"))))

(fiveam:test block-missing-brace
  (fiveam:is (null (block-from "{")))
  (fiveam:is (null (block-from "}"))))

(fiveam:test block-multi-statement
  (let ((ast (block-from "{ let x : int = 6; x = 7; return x; }")))
    (when (fiveam:is (block-node-p ast))
          (fiveam:is (= 3 (length (block-node-stmtlist ast)))))))
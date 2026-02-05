(in-package :acc)

(fiveam:def-suite program)
(fiveam:in-suite program)

(defun sequence-from (str)
  (fiveam:finishes
    (let ((tokens (tokenize str)))
      (make-token-sequence tokens))))

(defun program-from (str)
  (parse-program (sequence-from str)))

(fiveam:test program-basic
  (fiveam:is (program-node-p (program-from "fun main int { return 0; }"))))

(fiveam:test program-error
  (fiveam:signals error (program-from "return 0;")))

(defun fun-from (str)
  (fiveam:finishes (function-rule (sequence-from str))))

(fiveam:test function-basic
  (let ((ast (fun-from "fun main int { return 0; }")))
    (when (fiveam:is (function-node-p ast) "Function must be a function node")
          (fiveam:is (string= "main" (function-node-name ast)) "Function must be named main")
          (fiveam:is (integer-type-p (function-node-return-type ast))) "Function must return an integer")))

(fiveam:test function-failure
  (fiveam:is (null (fun-from "fun { return 0; }"))))

(defun stmt-from (str)
  (fiveam:finishes (stmt-rule (sequence-from str))))

(fiveam:test return
  (fiveam:is (return-statement-node-p (stmt-from "return 0;"))))

(fiveam:test declare
  (fiveam:is (declaration-node-p (stmt-from "let x : int = 0;"))))

(fiveam:test assign
  (fiveam:is (assignment-node-p (stmt-from "x = 1;"))))

(fiveam:test stmt-failure
  (fiveam:is (null (stmt-from "return return return"))))

(defun block-from (str)
  (fiveam:finishes (block-rule (sequence-from str))))

(fiveam:test block-empty
  (fiveam:is (block-node-p (block-from "{}"))))

(fiveam:test block-missing-brace
  (fiveam:is (null (block-from "{")) "Missing closing brace must fail")
  (fiveam:is (null (block-from "}")) "Missing open brace must fail"))

(fiveam:test block-multi-statement
  (let ((ast (block-from "{ let x : int = 6; x = 7; return x; }")))
    (when (fiveam:is (block-node-p ast) "Block must be a block node")
          (fiveam:is (= 3 (length (block-node-stmtlist ast))) "Block must have three statements"))))
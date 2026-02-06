(in-package :acc/test)

(defun sequence-from (str)
  (fiveam:finishes
   (let ((tokens (acc::tokenize str)))
     (acc::make-token-sequence tokens))))

(defun program-from (str)
  (acc::parse-program (sequence-from str)))

(defun fun-from (str)
  (fiveam:finishes (acc::function-rule (sequence-from str))))

(defun stmt-from (str)
  (fiveam:finishes (acc::stmt-rule (sequence-from str))))

(defun block-from (str)
  (fiveam:finishes (acc::block-rule (sequence-from str))))

(defun type-from (str)
  (acc::parse-type (sequence-from str)))

(defun expr-from (str)
  (acc::expr-bp (sequence-from str) 0))

(defun typed-expr-from (str)
  (acc::assign-type (fiveam:finishes (expr-from str)) (acc::make-env)))

(defun typed-stmt-from (str &key (env (acc::make-env)))
  (acc::assign-type (fiveam:finishes (stmt-from str)) env))

(defun typed-fun-from (str)
  (acc::assign-type (fiveam:finishes (fun-from str)) (acc::make-env)))

(defun typed-program-from (str)
  (acc::set-program-types (fiveam:finishes (program-from str))))

(defun expr-instrs-from (str)
  (acc::gen-expr (fiveam:finishes (typed-expr-from str))))

(defun trimmed-string-from (x)
  (string-trim '(#\Tab) (acc::to-string x)))
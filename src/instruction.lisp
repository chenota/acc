(in-package :acc)

;; HELPER MACROS

(defmacro def-atomic-operand (name control-string value-type)
  "Create a new atomic operand."
  (let* ((pkg (symbol-package name))
         (ctor (intern (format nil "MAKE-~A" (symbol-name name)) pkg)))
    `(progn
      (defclass ,name (atomic-operand) ())
      (defmethod control-string ((_ ,name)) (declare (ignore _)) ,control-string)
      (defmethod value-type ((_ ,name)) (declare (ignore _)) ',value-type)
      (defun ,ctor (value) (make-instance ',name :value value)))))

;; OPERANDS

(defclass operand () ()
  (:documentation "Operand base class."))

(defmethod print-operand :before ((o operand) s)
  "Assert the printer function writes to a stream."
  (assert (typep s 'stream)))

(defmethod to-string ((o operand))
  "Convert operand to a string."
  (with-output-to-string (s)
    (print-operand o s)))

(defclass atomic-operand (operand)
    ((value :initarg :value :accessor operand-value))
  (:documentation "Operand with a single item of structured data."))

(defmethod initialize-instance :before ((a atomic-operand) &key value &allow-other-keys)
  "Assert the type of the atomic operands value."
  (assert (typep value (value-type a))))

(defmethod print-operand ((a atomic-operand) s)
  "Print the atomic operand."
  (format s (control-string a) (operand-value a)))

(def-atomic-operand string-operand "~s" string)
(def-atomic-operand ident-operand "~a" string)
(def-atomic-operand type-operand "@~a" string)

;; INSTRUCTIONS

(defclass instruction ()
    ((operation :initarg :op :type string :accessor instr-op)
     (operands :initarg :oprs :type sequence :accessor instr-oprs :initform nil)
     (indent :initarg :indent :type boolean :accessor instr-indent :initform t)))

(defun make-instruction (operator &rest operands &key (indent t))
  (make-instance 'instruction :op operator :indent indent :oprs operands))

(defmethod print-instruction ((i instruction) s)
  (check-type s stream)
  (when (instr-indent i) (format s "~c" #\Tab))
  (format s "~a" (instr-op i))
  (let ((opr-count (length (instr-oprs i))))
    (when (> opr-count 0) (write-string " " s))
    (loop
   for operand in (instr-oprs i)
   for j = 0 then (1+ j)
   do (print-operand operand s)
     when (< j (1- opr-count)) do (write-string ", " s))))

(defmethod to-string ((i instruction))
  (with-output-to-string (s)
    (print-instruction i s)))
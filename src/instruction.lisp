(in-package :acc)

(defmacro def-atomic-operand (name control-string value-type)
  (let* ((pkg (symbol-package name))
         (ctor (intern (format nil "MAKE-~A" (symbol-name name)) pkg)))
    `(progn
      (defclass ,name (atomic-operand) ())
      (defmethod control-string ((x ,name)) ,control-string)
      (defmethod value-type ((x ,name)) ',value-type)
      (defun ,ctor (value) (make-instance ',name :value value)))))

(defclass operand () ())

(defmethod print-operand :before ((o operand) s)
  (check-type s stream))

(defmethod print-operand ((o operand) s)
  (format s "UNDEFINED"))

(defmethod to-string ((o operand))
  (with-output-to-string (s)
    (print-operand o s)))

(defclass atomic-operand (operand)
    ((value :initarg :value :accessor operand-value)))

(defmethod initialize-instance :after ((a atomic-operand) &key value &allow-other-keys)
  (assert (typep value (value-type a))))

(defmethod control-string ((a atomic-operand)) "~a")

(defmethod value-type ((a atomic-operand)) t)

(defmethod print-operand ((a atomic-operand) s)
  (format s (control-string a) (operand-value a)))

(def-atomic-operand string-operand "~s" string)

(def-atomic-operand ident-operand "~a" string)

(def-atomic-operand type-operand "@~a" string)

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
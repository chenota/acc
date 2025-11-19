(in-package :acc)

;; HELPER MACROS

(defmacro def-atomic-operand (name control-string value-type)
  "Create a new atomic operand."
  (let* ((pkg (symbol-package name))
         (make-func (intern (format nil "MAKE-~A" (symbol-name name)) pkg)))
    `(progn
      (defclass ,name (atomic-operand) ())
      (defmethod control-string ((_ ,name)) (declare (ignore _)) ,control-string)
      (defmethod value-type ((_ ,name)) (declare (ignore _)) ',value-type)
      (defun ,make-func (value) (make-instance ',name :value value)))))

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

(defclass line-item () ())

(defmethod to-string ((l line-item))
  "Convert line item to a string."
  (with-output-to-string (s)
    (print-line-item l s)))

(defmethod print-line-item :before ((l line-item) s)
  "Assert the line item writes to a stream."
  (assert (typep s 'stream)))

(defclass instruction (line-item)
    ((operation :initarg :op :type string :accessor instr-op)
     (operands :initarg :oprs :type sequence :accessor instr-oprs :initform nil))
  (:documentation "Basic x86 instruction with a type and operands."))

(defun make-instruction (operator &rest operands)
  (make-instance 'instruction :op operator :oprs operands))

(defmethod print-line-item ((i instruction) s)
  (format s "~c~a" #\tab (instr-op i))
  (let ((opr-count (length (instr-oprs i))))
    (when (> opr-count 0) (write-string " " s))
    (loop
   for operand in (instr-oprs i)
   for j = 0 then (1+ j)
   do (print-operand operand s)
     when (< j (1- opr-count)) do (write-string ", " s))))

(defmethod initialize-instance :before ((i instruction) &key oprs &allow-other-keys)
  "Assert that the instruction operands are only of the operand type."
  (assert (every (lambda (x) (typep x 'operand)) oprs)))

(defclass label (line-item)
    ((value :initarg :value :type string :accessor label-value))
  (:documentation "x86 label."))

(defun make-label (value)
  (make-instance 'label :value value))

(defmethod print-line-item ((l label) s)
  (format s "~a:" (label-value l)))
document.addEventListener('DOMContentLoaded', function () {
          document.getElementById('cg_members').addEventListener('input', groupadd_members);
          document.getElementById('cg_members_remove').addEventListener('input', groupremove_members);
	  document.getElementById('btn_form_modal_addmember').addEventListener('click', addmember_form_submit);
	  document.getElementById('btn_form_modal_removemember').addEventListener('click', removemember_form_submit);
	  var adminJoinButton = document.getElementById('btn_joingroup_admin');
          if (adminJoinButton != null) {
		  adminJoinButton.addEventListener('click', joingroup_admin);
          }
})

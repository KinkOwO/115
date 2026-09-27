// caller_sub_1451AAA80_0x1451aaa80

void __fastcall sub_1451AAA80(__int64 a1, __int64 a2)
{
  int v4; // esi
  __int64 v5; // rdx
  int v6; // eax
  __int64 v7; // rcx
  char v8; // bl
  __int64 v9; // rax

  if ( a2 )
  {
    v4 = sub_145AD5C20(qword_14E683C80, a2);
    if ( v4 >= 0 )
    {
      v5 = *(unsigned int *)(a1 + 288);
      if ( (int)v5 >= 0 )
      {
        v6 = sub_145AD55B0(qword_14E683C80, v5);
        if ( v6 < 0 )
        {
          if ( !qword_14E683C78 )
            return;
          v7 = 44327;
LABEL_10:
          v9 = sub_14723C170(v7);
          sub_14668C520(qword_14E683C78, 2875, v9, 0);
          return;
        }
        sub_142581F20(qword_14E683C80, v6);
      }
      sub_1450146B0(a2);
      v8 = sub_145AE9AE0(qword_14E683C80, (unsigned int)v4);
      sub_142581F20(qword_14E683C80, 0xFFFF);
      if ( v8 )
        return;
      v7 = 1659;
      goto LABEL_10;
    }
  }
}

